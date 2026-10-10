import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { authStore } from '@/store/auth';
import { loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { WalkInDialog } from './WalkInDialog';

afterEach(() => authStore.reset());
const URL = '*/api/v1/walk-ins';

describe('WalkInDialog（REQ-008 当日受付）', () => {
  it('未ログインならログインを案内し、API は呼ばない', async () => {
    authStore.clear();
    let called = false;
    server.use(http.post(URL, () => { called = true; return HttpResponse.json({}, { status: 201 }); }));
    renderWithProviders(<WalkInDialog department={{ id: 1, name: '内科' }} onClose={() => {}} />);

    expect(screen.getByRole('link', { name: 'ログインする' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '受付する' })).not.toBeInTheDocument();
    expect(called).toBe(false);
  });

  it('受付すると受付番号と前の人数を表示する', async () => {
    loginAs('patient');
    let body: unknown;
    server.use(http.post(URL, async ({ request }) => { body = await request.json(); return HttpResponse.json({ queue_entry_id: 'q', ticket_no: 7, ahead_count: 3 }, { status: 201 }); }));
    renderWithProviders(<WalkInDialog department={{ id: 1, name: '内科' }} onClose={() => {}} />);

    await userEvent.setup().click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByText('受付番号 7')).toBeInTheDocument();
    expect(screen.getByText(/あと3人/)).toBeInTheDocument();
    expect(body).toEqual({ department_id: 1 });
  });

  it('前に誰もいなければ「まもなくお呼びします」', async () => {
    loginAs('patient');
    server.use(http.post(URL, () => HttpResponse.json({ queue_entry_id: 'q', ticket_no: 1, ahead_count: 0 }, { status: 201 })));
    renderWithProviders(<WalkInDialog department={{ id: 1, name: '内科' }} onClose={() => {}} />);

    await userEvent.setup().click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByText(/まもなくお呼びします/)).toBeInTheDocument();
  });

  it.each([
    [422, { error: 'walkin_closed', message: 'walk-in registration is closed (after the cutoff time)' }, '受付時間'],
    [422, { error: 'walkin_closed', message: 'walk-in registration is closed (capacity reached)' }, '定員'],
    [409, { error: 'already_in_queue' }, '既に'],
  ])('%i %j は理由を伝える', async (status, body, expected) => {
    loginAs('patient');
    server.use(http.post(URL, () => HttpResponse.json(body, { status })));
    renderWithProviders(<WalkInDialog department={{ id: 1, name: '内科' }} onClose={() => {}} />);

    await userEvent.setup().click(screen.getByRole('button', { name: '受付する' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(expected);
  });

  it('Esc で閉じられる', async () => {
    loginAs('patient');
    const onClose = vi.fn();
    renderWithProviders(<WalkInDialog department={{ id: 1, name: '内科' }} onClose={onClose} />);

    await userEvent.setup().keyboard('{Escape}');

    expect(onClose).toHaveBeenCalled();
  });
});
