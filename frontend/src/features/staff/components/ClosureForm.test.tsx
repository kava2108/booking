import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { dept, doctor, loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { ClosureForm } from './ClosureForm';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

function setup() {
  loginAs('staff');
  server.use(
    http.get(`${BASE}/departments`, () => HttpResponse.json([dept(1, '内科')])),
    http.get(`${BASE}/doctors`, () => HttpResponse.json([doctor(7, '山田', 1), doctor(8, '佐藤', 1)])),
  );
  renderWithProviders(<ClosureForm departmentId={1} />);
}

async function fillRange(user: ReturnType<typeof userEvent.setup>, from: string, to: string) {
  await user.type(screen.getByLabelText('開始'), from);
  await user.type(screen.getByLabelText('終了'), to);
}

describe('ClosureForm（休診・医師不在の設定、REQ-005-AC-3）', () => {
  it('医療機関全体または医師を選べる', async () => {
    setup();

    const select = await screen.findByLabelText('対象');
    expect(within(select).getByRole('option', { name: '医療機関全体' })).toBeInTheDocument();
    expect(await within(select).findByRole('option', { name: '山田' })).toBeInTheDocument();
    expect(within(select).getByRole('option', { name: '佐藤' })).toBeInTheDocument();
  });

  it('開始・終了が未入力ならエラー', async () => {
    setup();

    await userEvent.setup().click(await screen.findByRole('button', { name: '休診を設定する' }));

    expect(await screen.findByText('開始日時を入力してください')).toBeInTheDocument();
    expect(screen.getByText('終了日時を入力してください')).toBeInTheDocument();
  });

  it('終了が開始以前ならエラー', async () => {
    setup();
    const user = userEvent.setup();
    await screen.findByLabelText('対象');

    await fillRange(user, '2099-01-05T12:00', '2099-01-05T09:00');
    await user.click(screen.getByRole('button', { name: '休診を設定する' }));

    expect(await screen.findByText('終了は開始より後にしてください')).toBeInTheDocument();
  });

  it('設定の前に影響の説明を表示して確認する。戻れば何も実行しない', async () => {
    setup();
    let called = false;
    server.use(http.post(`${BASE}/staff/closures`, () => { called = true; return HttpResponse.json({ closed_slots: 0, notified: 0 }, { status: 201 }); }));
    const user = userEvent.setup();
    await screen.findByLabelText('対象');

    await fillRange(user, '2099-01-05T09:00', '2099-01-05T12:00');
    await user.click(screen.getByRole('button', { name: '休診を設定する' }));
    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('予約できなくなります');
    expect(dialog).toHaveTextContent('通知');
    await user.click(within(dialog).getByRole('button', { name: '戻る' }));

    expect(called).toBe(false);
  });

  it('確定すると日本時間(+09:00)の日時で送信し、閉じた枠数と通知数を表示する', async () => {
    setup();
    let body: Record<string, unknown> = {};
    server.use(http.post(`${BASE}/staff/closures`, async ({ request }) => { body = (await request.json()) as Record<string, unknown>; return HttpResponse.json({ closed_slots: 6, notified: 3 }, { status: 201 }); }));
    const user = userEvent.setup();
    await screen.findByRole('option', { name: '山田' });

    await user.selectOptions(screen.getByLabelText('対象'), '山田');
    await fillRange(user, '2099-01-05T09:00', '2099-01-05T12:00');
    await user.type(screen.getByLabelText(/理由/), '学会');
    await user.click(screen.getByRole('button', { name: '休診を設定する' }));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: '設定する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('6枠を休診にし、3人に通知しました');
    expect(body).toEqual({ doctor_id: 7, from_at: '2099-01-05T09:00:00+09:00', to_at: '2099-01-05T12:00:00+09:00', reason: '学会' });
  });

  it('医療機関全体の休診は doctor_id を送らない', async () => {
    setup();
    let body: Record<string, unknown> = {};
    server.use(http.post(`${BASE}/staff/closures`, async ({ request }) => { body = (await request.json()) as Record<string, unknown>; return HttpResponse.json({ closed_slots: 0, notified: 0 }, { status: 201 }); }));
    const user = userEvent.setup();
    await screen.findByLabelText('対象');

    await fillRange(user, '2099-01-05T00:00', '2099-01-06T00:00');
    await user.click(screen.getByRole('button', { name: '休診を設定する' }));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: '設定する' }));

    await screen.findByRole('status');
    expect(body).not.toHaveProperty('doctor_id');
  });

  it('サーバーの検証エラー（過去など 422）は画面に表示する', async () => {
    setup();
    server.use(http.post(`${BASE}/staff/closures`, () => HttpResponse.json({ error: 'validation_failed', details: [{ field: 'to_at', message: 'must be in the future' }] }, { status: 422 })));
    const user = userEvent.setup();
    await screen.findByLabelText('対象');

    await fillRange(user, '2099-01-05T09:00', '2099-01-05T12:00');
    await user.click(screen.getByRole('button', { name: '休診を設定する' }));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: '設定する' }));

    expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent('入力内容');
  });
});
