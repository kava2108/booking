import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { authStore } from '@/store/auth';
import { available, loginAs, renderWithProviders, unavailable, waitTimes } from '@/test/helpers';
import { server } from '@/test/server';
import { WAIT_TIME_POLL_MS } from '../api/useWaitTimes';
import { WaitTimeBoard } from './WaitTimeBoard';

const URL = '*/api/v1/wait-times';
beforeEach(() => authStore.clear());
afterEach(() => { vi.useRealTimers(); authStore.reset(); });

describe('WaitTimeBoard', () => {
  it('未ログインでも診療科ごとの待ち時間と、目安である旨の注記を表示する', async () => {
    server.use(http.get(URL, () => HttpResponse.json(waitTimes([available(), unavailable('closed')]))));
    renderWithProviders(<WaitTimeBoard />);

    expect(await screen.findByRole('heading', { name: '内科' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '小児科' })).toBeInTheDocument();
    expect(screen.getByText('約35分')).toBeInTheDocument();
    expect(screen.getByText(/待ち時間は目安です/)).toBeInTheDocument();
    expect(screen.getByText(/09:41/)).toBeInTheDocument();
  });

  it('読み込み中を表示する', () => {
    server.use(http.get(URL, async () => { await new Promise((r) => setTimeout(r, 200)); return HttpResponse.json(waitTimes([])); }));
    renderWithProviders(<WaitTimeBoard />);

    expect(screen.getByRole('status')).toHaveTextContent('読み込み中');
  });

  it('診療科が無いときは案内を表示する', async () => {
    server.use(http.get(URL, () => HttpResponse.json(waitTimes([]))));
    renderWithProviders(<WaitTimeBoard />);

    expect(await screen.findByText('現在表示できる診療科がありません')).toBeInTheDocument();
  });

  it('最初の取得に失敗したらエラーと再試行ボタンを表示する', async () => {
    let n = 0;
    server.use(http.get(URL, () => (n++ === 0 ? HttpResponse.error() : HttpResponse.json(waitTimes([available()])))));
    renderWithProviders(<WaitTimeBoard />);

    expect(await screen.findByRole('alert')).toHaveTextContent('通信');
    await userEvent.setup().click(screen.getByRole('button', { name: '再試行' }));

    expect(await screen.findByRole('heading', { name: '内科' })).toBeInTheDocument();
  });

  // REQ-004-AC-3: 30 秒間隔で自動更新
  it('30 秒ごとに自動で更新する', async () => {
    expect(WAIT_TIME_POLL_MS).toBe(30_000);
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let calls = 0;
    server.use(http.get(URL, () => { calls++; return HttpResponse.json(waitTimes([available({ wait_minutes: calls * 10 })])); }));
    renderWithProviders(<WaitTimeBoard />);
    expect(await screen.findByText('約10分')).toBeInTheDocument();

    await act(async () => { await vi.advanceTimersByTimeAsync(WAIT_TIME_POLL_MS + 100); });

    expect(await screen.findByText('約20分')).toBeInTheDocument();
    expect(calls).toBe(2);
  });

  // 取得失敗時は前回値と更新不可を表示する
  it('更新に失敗したら前回の値を残し、更新できなかったことを伝える', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let calls = 0;
    server.use(http.get(URL, () => (++calls === 1 ? HttpResponse.json(waitTimes([available({ wait_minutes: 35 })])) : HttpResponse.error())));
    renderWithProviders(<WaitTimeBoard />);
    expect(await screen.findByText('約35分')).toBeInTheDocument();

    await act(async () => { await vi.advanceTimersByTimeAsync(WAIT_TIME_POLL_MS + 100); });

    expect(await screen.findByRole('alert')).toHaveTextContent('最新の情報を取得できませんでした');
    expect(screen.getByText('約35分')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('09:41');
  });

  it('手動で更新できる', async () => {
    let calls = 0;
    server.use(http.get(URL, () => { calls++; return HttpResponse.json(waitTimes([available({ wait_minutes: calls * 5 })])); }));
    renderWithProviders(<WaitTimeBoard />);
    expect(await screen.findByText('約5分')).toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole('button', { name: '最新の情報に更新' }));

    expect(await screen.findByText('約10分')).toBeInTheDocument();
  });

  it('ログイン済みでなくても当日受付ボタンは表示され、ログインを案内する', async () => {
    server.use(http.get(URL, () => HttpResponse.json(waitTimes([available()]))));
    renderWithProviders(<WaitTimeBoard />);
    await screen.findByRole('heading', { name: '内科' });

    await userEvent.setup().click(screen.getByRole('button', { name: '内科の当日受付をする' }));

    expect(await screen.findByRole('dialog')).toHaveTextContent('ログイン');
    expect(screen.getByRole('link', { name: 'ログインする' })).toHaveAttribute('href', '/login');
  });

  it('算出不可の診療科では当日受付ボタンを出さない（休診・時間外の受付を促さない）', async () => {
    server.use(http.get(URL, () => HttpResponse.json(waitTimes([unavailable('closed')]))));
    renderWithProviders(<WaitTimeBoard />);
    await screen.findByRole('heading', { name: '小児科' });

    expect(screen.queryByRole('button', { name: /当日受付をする/ })).not.toBeInTheDocument();
  });

  it('個人を特定できる情報を画面に出さない', async () => {
    loginAs('patient');
    server.use(http.get(URL, () => HttpResponse.json(waitTimes([available()]))));
    const { container } = renderWithProviders(<WaitTimeBoard />);
    await screen.findByRole('heading', { name: '内科' });

    await waitFor(() => expect(container.textContent).not.toMatch(/受付番号|患者|症状/));
  });
});
