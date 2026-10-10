import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { dept, doctor, loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { ScheduleEditor } from './ScheduleEditor';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

const row = (over: Record<string, unknown> = {}) => ({ weekday: 1, start_time: '09:00', end_time: '12:00', slot_minutes: 30, capacity: 2, ...over });

function setup(schedules: unknown[] = [row()]) {
  loginAs('clinic_admin');
  server.use(
    http.get(`${BASE}/departments`, () => HttpResponse.json([dept(1, '内科')])),
    http.get(`${BASE}/doctors`, () => HttpResponse.json([doctor(7, '山田', 1)])),
    http.get(`${BASE}/admin/schedules`, () => HttpResponse.json({ doctor_id: 7, schedules })),
  );
  renderWithProviders(<ScheduleEditor />);
}

async function pickDoctor() {
  const user = userEvent.setup();
  await screen.findByRole('option', { name: '内科' });
  await user.selectOptions(screen.getByLabelText('診療科'), '内科');
  await screen.findByRole('option', { name: '山田' });
  await user.selectOptions(screen.getByLabelText('医師'), '山田');
  return user;
}

describe('ScheduleEditor（REQ-006-AC-1 診療時間・枠長・定員）', () => {
  it('医師を選ぶと現在のスケジュールを表示する', async () => {
    setup([row(), row({ weekday: 3, start_time: '14:00', end_time: '17:00', slot_minutes: 20, capacity: 1 })]);
    await pickDoctor();

    expect(await screen.findByLabelText('曜日（1行目）')).toHaveValue('1');
    expect(screen.getByLabelText('開始（1行目）')).toHaveValue('09:00');
    expect(screen.getByLabelText('終了（1行目）')).toHaveValue('12:00');
    expect(screen.getByLabelText('枠の長さ・分（1行目）')).toHaveValue(30);
    expect(screen.getByLabelText('定員（1行目）')).toHaveValue(2);
    expect(screen.getByLabelText('曜日（2行目）')).toHaveValue('3');
    expect(screen.getByLabelText('枠の長さ・分（2行目）')).toHaveValue(20);
  });

  it('時間帯を追加・削除できる', async () => {
    setup([row()]);
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '時間帯を追加' }));
    expect(screen.getByLabelText('曜日（2行目）')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '2行目を削除' }));

    expect(screen.queryByLabelText('曜日（2行目）')).not.toBeInTheDocument();
  });

  it('保存すると、診療時間・枠長・定員を送って枠の生成を伝える', async () => {
    setup([row()]);
    let body: unknown;
    server.use(http.put(`${BASE}/admin/schedules`, async ({ request }) => { body = await request.json(); return new HttpResponse(null, { status: 200 }); }));
    const user = await pickDoctor();
    const cap = await screen.findByLabelText('定員（1行目）');
    await user.clear(cap);
    await user.type(cap, '3');

    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('保存しました');
    expect(body).toEqual({ doctor_id: 7, schedules: [{ weekday: 1, start_time: '09:00', end_time: '12:00', slot_minutes: 30, capacity: 3 }] });
  });

  it('終了が開始以前ならエラー（送信しない）', async () => {
    setup([row({ start_time: '12:00', end_time: '09:00' })]);
    let called = false;
    server.use(http.put(`${BASE}/admin/schedules`, () => { called = true; return new HttpResponse(null, { status: 200 }); }));
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByText('終了は開始より後にしてください')).toBeInTheDocument();
    expect(called).toBe(false);
  });

  it('枠の長さが診療時間より長ければエラー', async () => {
    setup([row({ start_time: '09:00', end_time: '09:20', slot_minutes: 30 })]);
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByText('枠の長さが診療時間を超えています')).toBeInTheDocument();
  });

  it('同じ曜日で時間帯が重なっていればエラー。隣接は可', async () => {
    setup([row({ start_time: '09:00', end_time: '12:00' }), row({ start_time: '11:00', end_time: '14:00' })]);
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '保存する' }));
    expect(await screen.findByText('同じ曜日の他の時間帯と重なっています')).toBeInTheDocument();

    const start2 = screen.getByLabelText('開始（2行目）');
    await user.clear(start2);
    await user.type(start2, '12:00');
    let saved = false;
    server.use(http.put(`${BASE}/admin/schedules`, () => { saved = true; return new HttpResponse(null, { status: 200 }); }));
    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('保存しました');
    expect(saved).toBe(true);
  });

  it('曜日が違えば同じ時間帯でも重複ではない', async () => {
    setup([row({ weekday: 1 }), row({ weekday: 2 })]);
    server.use(http.put(`${BASE}/admin/schedules`, () => new HttpResponse(null, { status: 200 })));
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('保存しました');
  });

  it('時間帯をすべて削除して保存すると「診療なし」になる旨を警告して確認する', async () => {
    setup([row()]);
    let body: unknown;
    server.use(http.put(`${BASE}/admin/schedules`, async ({ request }) => { body = await request.json(); return new HttpResponse(null, { status: 200 }); }));
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '1行目を削除' }));
    await user.click(screen.getByRole('button', { name: '保存する' }));
    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('診療なし');
    await user.click(within(dialog).getByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('保存しました');
    expect(body).toEqual({ doctor_id: 7, schedules: [] });
  });

  it('サーバーの検証エラー（422）を表示する', async () => {
    setup([row()]);
    server.use(http.put(`${BASE}/admin/schedules`, () => HttpResponse.json({ error: 'validation_failed', details: [{ field: 'schedules[0].end_time', message: 'must be after start_time' }] }, { status: 422 })));
    const user = await pickDoctor();
    await screen.findByLabelText('曜日（1行目）');

    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('入力内容');
  });

  it('医師を選ぶまでは編集欄を表示しない', async () => {
    setup();

    expect(await screen.findByLabelText('診療科')).toBeInTheDocument();
    expect(screen.getByText('診療科と医師を選ぶと、診療スケジュールを編集できます')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '保存する' })).not.toBeInTheDocument();
  });
});
