import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { SettingsForm } from './SettingsForm';

const BASE = '*/api/v1/admin/settings';
afterEach(() => authStore.reset());

const current = (over: Record<string, unknown> = {}) => ({
  cancel_deadline_minutes: 60,
  booking_window_days: 30,
  departments: [{ id: 1, name: '内科', standard_consult_minutes: 15, walkin_cutoff: '11:30', walkin_capacity: 20 }, { id: 2, name: '小児科', standard_consult_minutes: null, walkin_cutoff: null, walkin_capacity: null }],
  ...over,
});

describe('SettingsForm（REQ-006-AC-2 受付期間・キャンセル期限・標準診察時間）', () => {
  it('現在の設定値を表示する（未設定は空欄）', async () => {
    loginAs('clinic_admin');
    server.use(http.get(BASE, () => HttpResponse.json(current())));
    renderWithProviders(<SettingsForm />);

    expect(await screen.findByLabelText(/キャンセル期限/)).toHaveValue(60);
    expect(screen.getByLabelText(/予約受付期間/)).toHaveValue(30);
    expect(screen.getByLabelText('内科の標準診察時間（分）')).toHaveValue(15);
    expect(screen.getByLabelText('内科の当日受付の締切時刻')).toHaveValue('11:30');
    expect(screen.getByLabelText('内科の当日受付の定員（人）')).toHaveValue(20);
    expect(screen.getByLabelText('小児科の標準診察時間（分）')).toHaveValue(null);
  });

  it('保存すると変更した値を送る。空欄の項目は未設定として送らない', async () => {
    loginAs('clinic_admin');
    let body: unknown;
    server.use(
      http.get(BASE, () => HttpResponse.json(current())),
      http.put(BASE, async ({ request }) => { body = await request.json(); return HttpResponse.json({ cancel_deadline_minutes: 120, booking_window_days: 30 }); }),
    );
    renderWithProviders(<SettingsForm />);
    const user = userEvent.setup();

    const deadline = await screen.findByLabelText(/キャンセル期限/);
    await user.clear(deadline);
    await user.type(deadline, '120');
    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('保存しました');
    expect(body).toEqual({
      cancel_deadline_minutes: 120,
      booking_window_days: 30,
      departments: [
        { id: 1, standard_consult_minutes: 15, walkin_cutoff: '11:30', walkin_capacity: 20 },
        { id: 2 },
      ],
    });
  });

  it.each([
    ['キャンセル期限', '-1', '0以上で入力してください'],
    ['キャンセル期限', '44641', '44640以下で入力してください'],
    ['予約受付期間', '0', '1以上365以下で入力してください'],
    ['予約受付期間', '366', '1以上365以下で入力してください'],
  ])('%s に %s は範囲外', async (label, value, message) => {
    loginAs('clinic_admin');
    let called = false;
    server.use(http.get(BASE, () => HttpResponse.json(current())), http.put(BASE, () => { called = true; return HttpResponse.json({}); }));
    renderWithProviders(<SettingsForm />);
    const user = userEvent.setup();

    const field = await screen.findByLabelText(new RegExp(label));
    await user.clear(field);
    await user.type(field, value);
    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(called).toBe(false);
  });

  it('標準診察時間・定員は 1 以上、締切時刻は空欄または時刻', async () => {
    loginAs('clinic_admin');
    server.use(http.get(BASE, () => HttpResponse.json(current())));
    renderWithProviders(<SettingsForm />);
    const user = userEvent.setup();

    const std = await screen.findByLabelText('内科の標準診察時間（分）');
    await user.clear(std);
    await user.type(std, '0');
    const cap = screen.getByLabelText('内科の当日受付の定員（人）');
    await user.clear(cap);
    await user.type(cap, '0');
    await user.click(screen.getByRole('button', { name: '保存する' }));

    expect(await screen.findAllByText('1以上で入力してください')).toHaveLength(2);
  });

  it('サーバーの検証エラー（422）を表示する', async () => {
    loginAs('clinic_admin');
    server.use(http.get(BASE, () => HttpResponse.json(current())), http.put(BASE, () => HttpResponse.json({ error: 'validation_failed' }, { status: 422 })));
    renderWithProviders(<SettingsForm />);

    await userEvent.setup().click(await screen.findByRole('button', { name: '保存する' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('入力内容');
  });

  it('読み込みに失敗したらエラーと再試行を表示する', async () => {
    loginAs('clinic_admin');
    let n = 0;
    server.use(http.get(BASE, () => (n++ === 0 ? HttpResponse.json({ error: 'forbidden' }, { status: 403 }) : HttpResponse.json(current()))));
    renderWithProviders(<SettingsForm />);

    expect(await screen.findByRole('alert')).toHaveTextContent('権限');
    await userEvent.setup().click(screen.getByRole('button', { name: '再試行' }));

    expect(await screen.findByLabelText(/キャンセル期限/)).toBeInTheDocument();
  });
});
