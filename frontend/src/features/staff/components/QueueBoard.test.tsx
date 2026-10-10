import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { authStore } from '@/store/auth';
import { loginAs, queueEntry, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { STAFF_POLL_MS } from '../api/useStaff';
import { QueueBoard } from './QueueBoard';

const BASE = '*/api/v1';
afterEach(() => { vi.useRealTimers(); authStore.reset(); });

describe('QueueBoard（REQ-005 待機列）', () => {
  it('待機列を受付番号順に、氏名・種別・状態・症状つきで表示する', async () => {
    loginAs('staff');
    server.use(http.get(`${BASE}/staff/queue`, () => HttpResponse.json([
      queueEntry('q1', { ticket_no: 1, patient_name: '山田 太郎', source: 'reservation', status: 'in_consultation', symptom_note: '咳' }),
      queueEntry('q2', { ticket_no: 2, patient_name: '電話 花子', source: 'walk_in', status: 'waiting', symptom_note: '' }),
    ])));
    renderWithProviders(<QueueBoard departmentId={1} />);

    const rows = await screen.findAllByRole('row');
    expect(within(rows[1]!).getByText('山田 太郎')).toBeInTheDocument();
    expect(within(rows[1]!).getByText('予約')).toBeInTheDocument();
    expect(within(rows[1]!).getByText('診察中')).toBeInTheDocument();
    expect(within(rows[1]!).getByText('咳')).toBeInTheDocument();
    expect(within(rows[2]!).getByText('当日受付')).toBeInTheDocument();
    expect(within(rows[2]!).getByText('待機中')).toBeInTheDocument();
  });

  it('状態に応じた 1 つの操作ボタンだけを出す（呼出 → 診察開始 → 診察完了）', async () => {
    loginAs('staff');
    server.use(http.get(`${BASE}/staff/queue`, () => HttpResponse.json([
      queueEntry('q1', { ticket_no: 1, status: 'waiting' }),
      queueEntry('q2', { ticket_no: 2, status: 'called' }),
      queueEntry('q3', { ticket_no: 3, status: 'in_consultation' }),
    ])));
    renderWithProviders(<QueueBoard departmentId={1} />);

    const rows = await screen.findAllByRole('row');
    expect(within(rows[1]!).getByRole('button', { name: /呼出/ })).toBeInTheDocument();
    expect(within(rows[2]!).getByRole('button', { name: /診察開始/ })).toBeInTheDocument();
    expect(within(rows[3]!).getByRole('button', { name: /診察完了/ })).toBeInTheDocument();
    for (const r of rows.slice(1)) expect(within(r).getAllByRole('button')).toHaveLength(1);
  });

  it('呼出を押すと API を呼び、一覧を更新する', async () => {
    loginAs('staff');
    let status = 'waiting';
    let called = '';
    server.use(
      http.get(`${BASE}/staff/queue`, () => HttpResponse.json([queueEntry('q1', { status })])),
      http.post(`${BASE}/staff/queue/q1/call`, () => { called = 'call'; status = 'called'; return HttpResponse.json({ id: 'q1', status: 'called' }); }),
    );
    renderWithProviders(<QueueBoard departmentId={1} />);

    await userEvent.setup().click(await screen.findByRole('button', { name: /呼出/ }));

    expect(await screen.findByRole('button', { name: /診察開始/ })).toBeInTheDocument();
    expect(called).toBe('call');
  });

  it('診察開始・診察完了は対応するエンドポイントを呼ぶ', async () => {
    loginAs('staff');
    const paths: string[] = [];
    server.use(
      http.get(`${BASE}/staff/queue`, () => HttpResponse.json([queueEntry('q2', { status: 'called' }), queueEntry('q3', { ticket_no: 3, status: 'in_consultation' })])),
      http.post(`${BASE}/staff/queue/:id/:action`, ({ params }) => { paths.push(`${params.id}/${params.action}`); return HttpResponse.json({ id: params.id, status: 'x' }); }),
    );
    renderWithProviders(<QueueBoard departmentId={1} />);
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /診察開始/ }));
    await user.click(await screen.findByRole('button', { name: /診察完了/ }));

    expect(paths).toEqual(['q2/start', 'q3/complete']);
  });

  // 別の端末で先に操作された場合（409）は、その旨を伝えて最新の状態に更新する
  it('他のスタッフが先に操作していた（409）ら、メッセージを出して一覧を最新にする', async () => {
    loginAs('staff');
    let n = 0;
    server.use(
      http.get(`${BASE}/staff/queue`, () => HttpResponse.json([queueEntry('q1', { status: ++n === 1 ? 'waiting' : 'called' })])),
      http.post(`${BASE}/staff/queue/q1/call`, () => HttpResponse.json({ error: 'invalid_transition', message: 'cannot perform this operation in the current state (called)' }, { status: 409 })),
    );
    renderWithProviders(<QueueBoard departmentId={1} />);

    await userEvent.setup().click(await screen.findByRole('button', { name: /呼出/ }));

    expect(await screen.findByRole('alert')).toHaveTextContent('現在の状態ではこの操作はできません');
    expect(await screen.findByRole('button', { name: /診察開始/ })).toBeInTheDocument();
  });

  it('操作中はそのボタンを無効にして二重送信を防ぐ', async () => {
    loginAs('staff');
    let calls = 0;
    server.use(
      http.get(`${BASE}/staff/queue`, () => HttpResponse.json([queueEntry('q1')])),
      http.post(`${BASE}/staff/queue/q1/call`, async () => { calls++; await new Promise((r) => setTimeout(r, 100)); return HttpResponse.json({ id: 'q1', status: 'called' }); }),
    );
    renderWithProviders(<QueueBoard departmentId={1} />);
    const user = userEvent.setup();

    const btn = await screen.findByRole('button', { name: /呼出/ });
    await user.click(btn);
    expect(btn).toBeDisabled();
    await user.click(btn);

    expect(calls).toBe(1);
  });

  it('待っている人がいなければ案内する', async () => {
    loginAs('staff');
    server.use(http.get(`${BASE}/staff/queue`, () => HttpResponse.json([])));
    renderWithProviders(<QueueBoard departmentId={1} />);

    expect(await screen.findByText('現在、待機中の方はいません')).toBeInTheDocument();
  });

  it('権限エラー（403）は権限がない旨を伝える', async () => {
    loginAs('staff');
    server.use(http.get(`${BASE}/staff/queue`, () => HttpResponse.json({ error: 'forbidden' }, { status: 403 })));
    renderWithProviders(<QueueBoard departmentId={1} />);

    expect(await screen.findByRole('alert')).toHaveTextContent('権限');
  });

  it('定期的に自動更新する', async () => {
    expect(STAFF_POLL_MS).toBeLessThanOrEqual(30_000);
    loginAs('staff');
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let calls = 0;
    server.use(http.get(`${BASE}/staff/queue`, () => { calls++; return HttpResponse.json([queueEntry('q1', { patient_name: `患者${calls}` })]); }));
    renderWithProviders(<QueueBoard departmentId={1} />);
    expect(await screen.findByText('患者1')).toBeInTheDocument();

    await act(async () => { await vi.advanceTimersByTimeAsync(STAFF_POLL_MS + 100); });

    expect(await screen.findByText('患者2')).toBeInTheDocument();
  });

  it('診療科を指定して取得する', async () => {
    loginAs('staff');
    let q = '';
    server.use(http.get(`${BASE}/staff/queue`, ({ request }) => { q = new URL(request.url).search; return HttpResponse.json([]); }));
    renderWithProviders(<QueueBoard departmentId={3} />);

    await screen.findByText('現在、待機中の方はいません');
    expect(new URLSearchParams(q).get('department_id')).toBe('3');
  });
});
