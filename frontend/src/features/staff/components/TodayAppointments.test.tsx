import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { ymdJst } from '@/lib/format';
import { authStore } from '@/store/auth';
import { loginAs, renderWithProviders, staffAppt } from '@/test/helpers';
import { server } from '@/test/server';
import { TodayAppointments } from './TodayAppointments';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

describe('TodayAppointments（来院受付の入口）', () => {
  it('当日の予約を時刻順に、氏名・医師・来院区分つきで表示する', async () => {
    loginAs('staff');
    server.use(http.get(`${BASE}/staff/appointments`, () => HttpResponse.json([
      staffAppt('a1', { slot_start: '2026-10-12T09:30:00+09:00', patient_name: '山田 太郎', visit_type: 'first' }),
      staffAppt('a2', { slot_start: '2026-10-12T10:00:00+09:00', patient_name: '鈴木 花子', visit_type: 'revisit' }),
    ])));
    renderWithProviders(<TodayAppointments departmentId={1} />);

    const rows = await screen.findAllByRole('row');
    expect(rows[1]).toHaveTextContent('09:30');
    expect(rows[1]).toHaveTextContent('山田 太郎');
    expect(rows[1]).toHaveTextContent('初診');
    expect(rows[2]).toHaveTextContent('再診');
  });

  it('来院受付ボタンは「予約済み・未受付」の予約だけに出す', async () => {
    loginAs('staff');
    server.use(http.get(`${BASE}/staff/appointments`, () => HttpResponse.json([
      staffAppt('a1', { patient_name: '未受付' }),
      staffAppt('a2', { patient_name: '受付済み', checked_in: true }),
      staffAppt('a3', { patient_name: '診察済み', status: 'completed', checked_in: true }),
    ])));
    renderWithProviders(<TodayAppointments departmentId={1} />);

    const rows = await screen.findAllByRole('row');
    expect(within(rows[1]!).getByRole('button', { name: /来院受付/ })).toBeInTheDocument();
    expect(within(rows[2]!).queryByRole('button')).not.toBeInTheDocument();
    expect(within(rows[2]!).getByText('受付済み', { selector: 'span' })).toBeInTheDocument();
    expect(within(rows[3]!).queryByRole('button')).not.toBeInTheDocument();
    expect(within(rows[3]!).getByText('診察済み', { selector: 'span' })).toBeInTheDocument();
  });

  it('来院受付を押すと予約 ID で受付し、一覧が「受付済み」に更新される', async () => {
    loginAs('staff');
    let checkedIn = false;
    let path = '';
    server.use(
      http.get(`${BASE}/staff/appointments`, () => HttpResponse.json([staffAppt('a1', { patient_name: '山田 太郎', checked_in: checkedIn })])),
      http.post(`${BASE}/staff/queue/:id/check-in`, ({ params }) => { path = String(params.id); checkedIn = true; return HttpResponse.json({ id: 'q1', status: 'waiting' }); }),
    );
    renderWithProviders(<TodayAppointments departmentId={1} />);

    await userEvent.setup().click(await screen.findByRole('button', { name: /山田 太郎.*来院受付/ }));

    expect(await screen.findByText('受付済み', { selector: 'span' })).toBeInTheDocument();
    expect(path).toBe('a1');
  });

  it('二重の来院受付（409）は「既に受付済み」と案内する', async () => {
    loginAs('staff');
    server.use(
      http.get(`${BASE}/staff/appointments`, () => HttpResponse.json([staffAppt('a1')])),
      http.post(`${BASE}/staff/queue/a1/check-in`, () => HttpResponse.json({ error: 'already_in_queue' }, { status: 409 })),
    );
    renderWithProviders(<TodayAppointments departmentId={1} />);

    await userEvent.setup().click(await screen.findByRole('button', { name: /来院受付/ }));

    expect(await screen.findByRole('alert')).toHaveTextContent('既に');
  });

  it('当日でない・取消済みの予約（422）は理由を伝える', async () => {
    loginAs('staff');
    server.use(
      http.get(`${BASE}/staff/appointments`, () => HttpResponse.json([staffAppt('a1')])),
      http.post(`${BASE}/staff/queue/a1/check-in`, () => HttpResponse.json({ error: 'validation_failed', details: [{ field: 'id', message: 'appointment is not active: canceled' }] }, { status: 422 })),
    );
    renderWithProviders(<TodayAppointments departmentId={1} />);

    await userEvent.setup().click(await screen.findByRole('button', { name: /来院受付/ }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });

  it('今日の日付で取得し、予約が無ければ案内する', async () => {
    loginAs('staff');
    let q = '';
    server.use(http.get(`${BASE}/staff/appointments`, ({ request }) => { q = new URL(request.url).search; return HttpResponse.json([]); }));
    renderWithProviders(<TodayAppointments departmentId={2} />);

    expect(await screen.findByText('本日のご予約はありません')).toBeInTheDocument();
    const params = new URLSearchParams(q);
    expect(params.get('department_id')).toBe('2');
    expect(params.get('date')).toBe(ymdJst());
  });
});
