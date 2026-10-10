import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { StaffWalkInForm } from './StaffWalkInForm';

const URL_ = '*/api/v1/staff/walk-ins';
afterEach(() => authStore.reset());

describe('StaffWalkInForm（当日受付の代行・アカウントなしの患者）', () => {
  it('氏名が空なら送信せずエラー', async () => {
    loginAs('staff');
    let called = false;
    server.use(http.post(URL_, () => { called = true; return HttpResponse.json({}, { status: 201 }); }));
    renderWithProviders(<StaffWalkInForm departmentId={1} />);

    await userEvent.setup().click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByText('氏名を入力してください')).toBeInTheDocument();
    expect(called).toBe(false);
  });

  it('氏名は 100 文字以内', async () => {
    loginAs('staff');
    renderWithProviders(<StaffWalkInForm departmentId={1} />);
    const user = userEvent.setup();

    await user.click(screen.getByLabelText('氏名'));
    await user.paste('あ'.repeat(101));
    await user.click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByText('100文字以内で入力してください')).toBeInTheDocument();
  });

  it('受付すると受付番号と前の人数を表示し、続けて次の受付ができる', async () => {
    loginAs('staff');
    let body: unknown;
    server.use(http.post(URL_, async ({ request }) => { body = await request.json(); return HttpResponse.json({ queue_entry_id: 'q', ticket_no: 5, ahead_count: 2 }, { status: 201 }); }));
    renderWithProviders(<StaffWalkInForm departmentId={3} />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText('氏名'), '電話 太郎');
    await user.click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('受付番号 5');
    expect(screen.getByRole('status')).toHaveTextContent('あと2人');
    expect(body).toEqual({ department_id: 3, display_name: '電話 太郎' });
    expect(screen.getByLabelText('氏名')).toHaveValue('');
  });

  it.each([
    [{ error: 'walkin_closed', message: 'walk-in registration is closed (capacity reached)' }, 422, '定員'],
    [{ error: 'walkin_closed', message: 'walk-in registration is closed (after the cutoff time)' }, 422, '受付時間'],
  ])('締切・定員は理由を伝える %j', async (body, status, expected) => {
    loginAs('staff');
    server.use(http.post(URL_, () => HttpResponse.json(body, { status })));
    renderWithProviders(<StaffWalkInForm departmentId={1} />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText('氏名'), '電話 太郎');
    await user.click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(expected);
  });
});
