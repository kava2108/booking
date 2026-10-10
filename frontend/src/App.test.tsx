import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { AppRoutes } from './App';
import { authStore } from './store/auth';
import { available, dept, fakeJwt, loginAs, newQueryClient, waitTimes } from './test/helpers';
import { server } from './test/server';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

function renderApp(route: string) {
  server.use(
    http.get(`${BASE}/wait-times`, () => HttpResponse.json(waitTimes([available()]))),
    http.get(`${BASE}/departments`, () => HttpResponse.json([dept()])),
    http.get(`${BASE}/doctors`, () => HttpResponse.json([])),
  );
  return render(
    <QueryClientProvider client={newQueryClient()}>
      <MemoryRouter initialEntries={[route]}>
        <AppRoutes />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ルーティング', () => {
  it('トップは待ち時間（未ログインでも見られる）', async () => {
    authStore.clear();
    renderApp('/');

    expect(await screen.findByRole('heading', { level: 1, name: '現在の待ち時間' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: '内科' })).toBeInTheDocument();
  });

  it('予約画面は未ログインでも空き枠を探せる（予約の確定時にログインを求める）', () => {
    authStore.clear();
    renderApp('/reserve');

    expect(screen.getByRole('heading', { level: 1, name: '予約する' })).toBeInTheDocument();
  });

  it('マイページは未ログインならログイン画面へ', () => {
    authStore.clear();
    renderApp('/mypage');

    expect(screen.getByRole('heading', { level: 1, name: 'ログイン' })).toBeInTheDocument();
  });

  it.each(['/staff', '/admin'])('%s は患者には権限なしの案内を表示する', (path) => {
    loginAs('patient');
    renderApp(path);

    expect(screen.getByRole('alert')).toHaveTextContent('権限がありません');
  });

  it('スタッフ画面はスタッフに表示される', async () => {
    loginAs('staff');
    renderApp('/staff');

    expect(await screen.findByRole('heading', { level: 1, name: 'スタッフ画面' })).toBeInTheDocument();
  });

  it('管理画面はクリニック管理者に表示される。スタッフには表示されない', async () => {
    loginAs('staff');
    const { unmount } = renderApp('/admin');
    expect(screen.getByRole('alert')).toHaveTextContent('権限がありません');
    unmount();

    loginAs('clinic_admin');
    renderApp('/admin');
    expect(await screen.findByRole('heading', { level: 1, name: '管理画面' })).toBeInTheDocument();
  });

  it('ログイン済みでログイン・登録画面を開くと、それぞれの既定の画面へ移動する', async () => {
    loginAs('patient');
    server.use(http.get(`${BASE}/appointments/me`, () => HttpResponse.json([])));
    renderApp('/login');

    expect(await screen.findByRole('heading', { level: 1, name: 'マイページ' })).toBeInTheDocument();
  });

  it('存在しないページは 404 の案内とトップへのリンク', () => {
    authStore.clear();
    renderApp('/no/such/page');

    expect(screen.getByRole('heading', { level: 1, name: 'ページが見つかりません' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'トップへ戻る' })).toHaveAttribute('href', '/');
  });

  it('パスワード再設定の画面が開ける', () => {
    authStore.clear();
    renderApp('/password-reset');

    expect(screen.getByRole('heading', { level: 1, name: 'パスワードの再設定' })).toBeInTheDocument();
  });

  it('ページタイトルを画面ごとに更新する', async () => {
    authStore.clear();
    renderApp('/reserve');

    await waitFor(() => expect(document.title).toBe('予約する | クリニック予約'));
  });
});

describe('起動時のセッション復元', () => {
  it('リフレッシュ用 Cookie が有効なら、ログイン状態で起動する', async () => {
    const { App } = await import('./App');
    server.use(
      http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ access_token: fakeJwt({ role: 'patient' }), token_type: 'Bearer', expires_in: 900 })),
      http.get(`${BASE}/wait-times`, () => HttpResponse.json(waitTimes([]))),
    );
    window.history.pushState({}, '', '/');
    render(<App />);

    expect(await screen.findByRole('button', { name: 'ログアウト' })).toBeInTheDocument();
  });

  it('Cookie が無ければ未ログインで起動する（エラー表示はしない）', async () => {
    const { App } = await import('./App');
    server.use(
      http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ error: 'invalid_token' }, { status: 401 })),
      http.get(`${BASE}/wait-times`, () => HttpResponse.json(waitTimes([]))),
    );
    window.history.pushState({}, '', '/');
    render(<App />);

    expect(await screen.findByRole('link', { name: 'ログイン' })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
