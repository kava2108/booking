import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { ymdJst } from '@/lib/format';
import { authStore } from '@/store/auth';
import { appt, dept, doctor, loginAs, renderWithProviders, slot } from '@/test/helpers';
import { server } from '@/test/server';
import { BookingFlow } from './BookingFlow';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

function catalog(slots: unknown[] = [slot('s1', '2026-10-19T09:30:00+09:00'), slot('s2', '2026-10-19T10:00:00+09:00')]) {
  server.use(
    http.get(`${BASE}/departments`, () => HttpResponse.json([dept(1, '内科'), dept(2, '小児科')])),
    http.get(`${BASE}/doctors`, ({ request }) => HttpResponse.json(new URL(request.url).searchParams.get('department_id') === '1' ? [doctor(7, '山田', 1)] : [doctor(9, '鈴木', 2)])),
    http.get(`${BASE}/slots`, () => HttpResponse.json(slots)),
  );
}

async function chooseDepartment(name = '内科') {
  const user = userEvent.setup();
  await user.selectOptions(await screen.findByLabelText('診療科'), name);
  return user;
}

describe('BookingFlow（予約の流れ）', () => {
  it('診療科を選ぶまでは枠を表示せず、案内を出す', async () => {
    catalog();
    renderWithProviders(<BookingFlow />);

    expect(await screen.findByLabelText('診療科')).toBeInTheDocument();
    expect(screen.getByText('診療科を選ぶと、空き枠が表示されます')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /09:30/ })).not.toBeInTheDocument();
  });

  it('診療科を選ぶと医師を絞り込め、今日の空き枠を表示する', async () => {
    catalog();
    let slotQuery = '';
    server.use(http.get(`${BASE}/slots`, ({ request }) => { slotQuery = new URL(request.url).search; return HttpResponse.json([slot('s1', '2026-10-19T09:30:00+09:00')]); }));
    renderWithProviders(<BookingFlow />);

    await chooseDepartment('内科');

    expect(await screen.findByRole('button', { name: /09:30/ })).toBeInTheDocument();
    expect(await screen.findByRole('option', { name: '山田' })).toBeInTheDocument();
    expect(new URLSearchParams(slotQuery).get('date')).toBe(ymdJst());
  });

  it('医師を指名すると、その医師の枠で検索する', async () => {
    catalog();
    const queries: string[] = [];
    server.use(http.get(`${BASE}/slots`, ({ request }) => { queries.push(new URL(request.url).search); return HttpResponse.json([]); }));
    renderWithProviders(<BookingFlow />);
    const user = await chooseDepartment('内科');
    await screen.findByRole('option', { name: '山田' });

    await user.selectOptions(screen.getByLabelText('医師（任意）'), '山田');

    await waitFor(() => expect(new URLSearchParams(queries.at(-1)).get('doctor_id')).toBe('7'));
  });

  it('日付を切り替えられる。今日より前には戻れない', async () => {
    catalog();
    renderWithProviders(<BookingFlow />);
    const user = await chooseDepartment();

    expect(screen.getByRole('button', { name: '前の日' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '次の日' }));

    expect(screen.getByRole('button', { name: '前の日' })).toBeEnabled();
    expect(screen.getByLabelText('日付')).not.toHaveValue(ymdJst());
  });

  describe('予約の確定', () => {
    async function openConfirm() {
      catalog();
      renderWithProviders(<BookingFlow />);
      const user = await chooseDepartment();
      await user.click(await screen.findByRole('button', { name: /09:30/ }));
      return { user, dialog: await screen.findByRole('dialog') };
    }

    it('未ログインならログインを案内し、予約フォームは出さない', async () => {
      authStore.clear();
      const { dialog } = await openConfirm();

      expect(within(dialog).getByRole('link', { name: 'ログインする' })).toBeInTheDocument();
      expect(within(dialog).queryByRole('button', { name: '予約を確定する' })).not.toBeInTheDocument();
    });

    it('ログイン済みなら診療科・日時を確認して、来院区分を選んで確定できる', async () => {
      loginAs('patient');
      let body: unknown;
      server.use(http.post(`${BASE}/appointments`, async ({ request }) => { body = await request.json(); return HttpResponse.json({ id: 'a1', appointment_no: 'A-20261019-0042', slot_start: '2026-10-19T09:30:00+09:00', department: '内科', doctor: '山田' }, { status: 201 }); }));
      const { user, dialog } = await openConfirm();

      expect(dialog).toHaveTextContent('内科');
      expect(dialog).toHaveTextContent('山田');
      expect(dialog).toHaveTextContent('09:30');
      await user.click(within(dialog).getByLabelText('再診'));
      await user.type(within(dialog).getByLabelText(/症状など/), '3日前から咳');
      await user.click(within(dialog).getByRole('button', { name: '予約を確定する' }));

      expect(await screen.findByText('予約を受け付けました')).toBeInTheDocument();
      expect(screen.getByText('A-20261019-0042')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'マイページで確認する' })).toHaveAttribute('href', '/mypage');
      expect(body).toEqual({ slot_id: 's1', visit_type: 'revisit', symptom_note: '3日前から咳' });
    });

    it('来院区分が未選択なら確定できない', async () => {
      loginAs('patient');
      let called = false;
      server.use(http.post(`${BASE}/appointments`, () => { called = true; return HttpResponse.json({}, { status: 201 }); }));
      const { user, dialog } = await openConfirm();

      await user.click(within(dialog).getByRole('button', { name: '予約を確定する' }));

      expect(await within(dialog).findByText('初診か再診かを選んでください')).toBeInTheDocument();
      expect(called).toBe(false);
    });

    it('症状メモは 1000 文字まで。閲覧できる人の範囲を説明する', async () => {
      loginAs('patient');
      const { user, dialog } = await openConfirm();
      expect(dialog).toHaveTextContent('担当スタッフ');

      await user.click(within(dialog).getByLabelText('初診'));
      await user.click(within(dialog).getByLabelText(/症状など/));
      await user.paste('あ'.repeat(1001));
      await user.click(within(dialog).getByRole('button', { name: '予約を確定する' }));

      expect(await within(dialog).findByText('1000文字以内で入力してください')).toBeInTheDocument();
    });

    // REQ-002-AC-2: 満席になっていたら別の枠を選び直せる
    it('満席（409）ならメッセージを出し、枠の一覧を最新にして選び直せる', async () => {
      loginAs('patient');
      const { user, dialog } = await openConfirm();
      // 確定の直前に他の人が最後の 1 枠を取った状況: 以降の枠一覧では 09:30 が満席になっている
      server.use(
        http.post(`${BASE}/appointments`, () => HttpResponse.json({ error: 'slot_full', message: 'x' }, { status: 409 })),
        http.get(`${BASE}/slots`, () => HttpResponse.json([slot('s1', '2026-10-19T09:30:00+09:00', { available: false, unavailable_reason: 'full' }), slot('s2', '2026-10-19T10:00:00+09:00')])),
      );

      await user.click(within(dialog).getByLabelText('初診'));
      await user.click(within(dialog).getByRole('button', { name: '予約を確定する' }));

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('満席');
      await user.click(within(dialog).getByRole('button', { name: '別の枠を選ぶ' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(await screen.findByRole('button', { name: /09:30.*満席/ })).toBeDisabled();
      expect(screen.getByRole('button', { name: /10:00/ })).toBeEnabled();
    });

    it('同じ時間帯に予約済み（409）の案内', async () => {
      loginAs('patient');
      server.use(http.post(`${BASE}/appointments`, () => HttpResponse.json({ error: 'duplicate_appointment' }, { status: 409 })));
      const { user, dialog } = await openConfirm();

      await user.click(within(dialog).getByLabelText('初診'));
      await user.click(within(dialog).getByRole('button', { name: '予約を確定する' }));

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('同じ時間帯');
    });

    it('二重送信を防ぐ（送信中は無効）', async () => {
      loginAs('patient');
      let calls = 0;
      server.use(http.post(`${BASE}/appointments`, async () => { calls++; await new Promise((r) => setTimeout(r, 100)); return HttpResponse.json(appt('a1'), { status: 201 }); }));
      const { user, dialog } = await openConfirm();

      await user.click(within(dialog).getByLabelText('初診'));
      const submit = within(dialog).getByRole('button', { name: '予約を確定する' });
      await user.click(submit);
      expect(submit).toBeDisabled();
      await screen.findByText('予約を受け付けました');

      expect(calls).toBe(1);
    });
  });
});
