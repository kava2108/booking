import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { ymdJst } from '@/lib/format';
import { authStore } from '@/store/auth';
import { appt, dept, doctor, loginAs, renderWithProviders, slot } from '@/test/helpers';
import { server } from '@/test/server';
import { AppointmentList } from './AppointmentList';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

function list(items: unknown[]) {
  server.use(http.get(`${BASE}/appointments/me`, () => HttpResponse.json(items)));
}

describe('AppointmentList（REQ-003 マイページ）', () => {
  it('予約が無ければ案内を表示する', async () => {
    loginAs('patient');
    list([]);
    renderWithProviders(<AppointmentList />);

    expect(await screen.findByText('ご予約はありません')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '予約する' })).toHaveAttribute('href', '/reserve');
  });

  it('今後の予約と、過去・キャンセル済みを分けて表示する', async () => {
    loginAs('patient');
    list([
      appt('a1', { appointment_no: 'A-FUTURE', slot_start: '2099-01-05T09:30:00+09:00' }),
      appt('a2', { appointment_no: 'A-PAST', slot_start: '2020-01-05T09:30:00+09:00', status: 'completed', cancelable: false }),
      appt('a3', { appointment_no: 'A-CANCELED', slot_start: '2099-02-05T09:30:00+09:00', status: 'canceled', cancelable: false }),
    ]);
    renderWithProviders(<AppointmentList />);

    const upcoming = await screen.findByRole('region', { name: '今後のご予約' });
    expect(within(upcoming).getByText('A-FUTURE')).toBeInTheDocument();
    expect(within(upcoming).queryByText('A-CANCELED')).not.toBeInTheDocument();
    const history = screen.getByRole('region', { name: 'これまでのご予約' });
    expect(within(history).getByText('A-PAST')).toBeInTheDocument();
    expect(within(history).getByText('A-CANCELED')).toBeInTheDocument();
    expect(within(history).getByText('キャンセル済み')).toBeInTheDocument();
  });

  it('予約番号・日時・診療科・医師・来院区分・症状を本人に表示する', async () => {
    loginAs('patient');
    list([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00' })]);
    renderWithProviders(<AppointmentList />);

    const card = await screen.findByRole('article');
    expect(card).toHaveTextContent('A-20261019-0042');
    expect(card).toHaveTextContent('09:30');
    expect(card).toHaveTextContent('内科');
    expect(card).toHaveTextContent('山田');
    expect(card).toHaveTextContent('初診');
    expect(card).toHaveTextContent('咳');
  });

  describe('キャンセル（REQ-003-AC-2/3）', () => {
    it('期限内ならキャンセルでき、確認のうえ実行すると一覧が更新される', async () => {
      loginAs('patient');
      let canceled = false;
      server.use(
        http.get(`${BASE}/appointments/me`, () => HttpResponse.json([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00', status: canceled ? 'canceled' : 'booked', cancelable: !canceled })])),
        http.delete(`${BASE}/appointments/a1`, () => { canceled = true; return new HttpResponse(null, { status: 204 }); }),
      );
      renderWithProviders(<AppointmentList />);
      const user = userEvent.setup();

      await user.click(await screen.findByRole('button', { name: 'キャンセルする' }));
      const dialog = await screen.findByRole('dialog');
      expect(dialog).toHaveTextContent('A-20261019-0042');
      await user.click(within(dialog).getByRole('button', { name: 'キャンセルを確定する' }));

      expect(await screen.findByText('キャンセル済み')).toBeInTheDocument();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('確認画面で「戻る」を選べば何も実行しない', async () => {
      loginAs('patient');
      let called = false;
      list([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00' })]);
      server.use(http.delete(`${BASE}/appointments/a1`, () => { called = true; return new HttpResponse(null, { status: 204 }); }));
      renderWithProviders(<AppointmentList />);
      const user = userEvent.setup();

      await user.click(await screen.findByRole('button', { name: 'キャンセルする' }));
      await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: '戻る' }));

      expect(called).toBe(false);
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    // 期限超過は操作を出さず、電話での連絡を案内する
    it('期限を過ぎた予約はキャンセル・変更ボタンを出さず、医療機関への連絡を案内する', async () => {
      loginAs('patient');
      list([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00', cancelable: false })]);
      renderWithProviders(<AppointmentList />);

      const card = await screen.findByRole('article');
      expect(within(card).queryByRole('button', { name: 'キャンセルする' })).not.toBeInTheDocument();
      expect(within(card).queryByRole('button', { name: '日時を変更する' })).not.toBeInTheDocument();
      expect(card).toHaveTextContent('期限を過ぎています');
      expect(card).toHaveTextContent('お電話');
    });

    it('サーバーが期限超過（422）を返したらメッセージを表示する', async () => {
      loginAs('patient');
      list([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00' })]);
      server.use(http.delete(`${BASE}/appointments/a1`, () => HttpResponse.json({ error: 'cancel_deadline_passed' }, { status: 422 })));
      renderWithProviders(<AppointmentList />);
      const user = userEvent.setup();

      await user.click(await screen.findByRole('button', { name: 'キャンセルする' }));
      await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'キャンセルを確定する' }));

      expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent('期限');
    });
  });

  describe('日時の変更（REQ-003-AC-4）', () => {
    it('同じ診療科の別の枠を選んで変更できる', async () => {
      loginAs('patient');
      list([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00' })]);
      let body: unknown;
      server.use(
        http.get(`${BASE}/departments`, () => HttpResponse.json([dept(1, '内科')])),
        http.get(`${BASE}/doctors`, () => HttpResponse.json([doctor(7, '山田', 1)])),
        http.get(`${BASE}/slots`, () => HttpResponse.json([slot('new1', '2099-01-12T10:00:00+09:00')])),
        http.patch(`${BASE}/appointments/a1`, async ({ request }) => { body = await request.json(); return HttpResponse.json({ id: 'a1', appointment_no: 'A-20261019-0042', slot_start: '2099-01-12T10:00:00+09:00', department: '内科', doctor: '山田' }); }),
      );
      renderWithProviders(<AppointmentList />);
      const user = userEvent.setup();

      await user.click(await screen.findByRole('button', { name: '日時を変更する' }));
      const dialog = await screen.findByRole('dialog');
      await user.click(await within(dialog).findByRole('button', { name: /10:00/ }));
      await user.click(within(dialog).getByRole('button', { name: 'この日時に変更する' }));

      expect(await screen.findByText('日時を変更しました')).toBeInTheDocument();
      expect(body).toEqual({ slot_id: 'new1' });
    });

    it('変更先が満席（409）ならメッセージを表示し、元の予約は変わらない', async () => {
      loginAs('patient');
      list([appt('a1', { slot_start: '2099-01-05T09:30:00+09:00' })]);
      server.use(
        http.get(`${BASE}/departments`, () => HttpResponse.json([dept(1, '内科')])),
        http.get(`${BASE}/doctors`, () => HttpResponse.json([doctor(7, '山田', 1)])),
        http.get(`${BASE}/slots`, () => HttpResponse.json([slot('new1', '2099-01-12T10:00:00+09:00')])),
        http.patch(`${BASE}/appointments/a1`, () => HttpResponse.json({ error: 'slot_full' }, { status: 409 })),
      );
      renderWithProviders(<AppointmentList />);
      const user = userEvent.setup();

      await user.click(await screen.findByRole('button', { name: '日時を変更する' }));
      const dialog = await screen.findByRole('dialog');
      await user.click(await within(dialog).findByRole('button', { name: /10:00/ }));
      await user.click(within(dialog).getByRole('button', { name: 'この日時に変更する' }));

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('満席');
    });
  });

  describe('当日の順番表示（REQ-004-AC-4）', () => {
    it('本日の有効な予約には順番パネルを表示し、それ以外には出さない', async () => {
      loginAs('patient');
      const today = ymdJst();
      list([
        appt('today', { appointment_no: 'A-TODAY', slot_start: `${today}T23:30:00+09:00` }),
        appt('later', { appointment_no: 'A-LATER', slot_start: '2099-01-05T09:30:00+09:00' }),
      ]);
      let asked: string[] = [];
      server.use(http.get(`${BASE}/appointments/:id/position`, ({ params }) => { asked.push(String(params.id)); return HttpResponse.json({ status: 'waiting', ahead_count: 2, estimated_call_at: null }); }));
      renderWithProviders(<AppointmentList />);

      expect(await screen.findByText(/あと2人/)).toBeInTheDocument();
      expect(asked).toEqual(['today']);
      asked = [];
    });
  });

  it('取得に失敗したらエラーと再試行を表示する', async () => {
    loginAs('patient');
    let n = 0;
    server.use(http.get(`${BASE}/appointments/me`, () => (n++ === 0 ? HttpResponse.error() : HttpResponse.json([]))));
    renderWithProviders(<AppointmentList />);

    expect(await screen.findByRole('alert')).toHaveTextContent('通信');
    await userEvent.setup().click(screen.getByRole('button', { name: '再試行' }));

    expect(await screen.findByText('ご予約はありません')).toBeInTheDocument();
  });
});
