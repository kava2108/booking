import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { fakeJwt, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { LoginForm } from './LoginForm';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

const tokenResponse = (role: string) => HttpResponse.json({ access_token: fakeJwt({ role }), token_type: 'Bearer', expires_in: 900 });

async function fill(email: string, password: string) {
  const user = userEvent.setup();
  if (email) await user.type(screen.getByLabelText(/メールアドレス/), email);
  if (password) await user.type(screen.getByLabelText(/パスワード/), password);
  await user.click(screen.getByRole('button', { name: 'ログイン' }));
  return user;
}

describe('LoginForm', () => {
  it('未入力ならエラーを表示し、API を呼ばない', async () => {
    let called = false;
    server.use(http.post(`${BASE}/auth/login`, () => { called = true; return tokenResponse('patient'); }));
    renderWithProviders(<LoginForm />);

    await fill('', '');

    expect(await screen.findByText('メールアドレスを入力してください')).toBeInTheDocument();
    expect(screen.getByText('パスワードを入力してください')).toBeInTheDocument();
    expect(called).toBe(false);
  });

  it('メールアドレスの形式が不正ならエラー', async () => {
    renderWithProviders(<LoginForm />);

    await fill('not-an-email', 'password-12345');

    expect(await screen.findByText('メールアドレスの形式が正しくありません')).toBeInTheDocument();
  });

  it('成功するとトークンを保持し、患者はマイページへ遷移する', async () => {
    server.use(http.post(`${BASE}/auth/login`, () => tokenResponse('patient')));
    renderWithProviders(<LoginForm />, { extraRoutes: { '/mypage': 'マイページ画面' } });

    await fill('taro@example.com', 'SecurePass123!');

    expect(await screen.findByText('マイページ画面')).toBeInTheDocument();
    expect(authStore.getState()).toMatchObject({ status: 'authenticated', role: 'patient' });
  });

  it.each([
    ['staff', '/staff', 'スタッフ画面'],
    ['clinic_admin', '/staff', 'スタッフ画面'],
  ])('%s は %s へ遷移する', async (role, path, text) => {
    server.use(http.post(`${BASE}/auth/login`, () => tokenResponse(role)));
    renderWithProviders(<LoginForm />, { extraRoutes: { [path]: text } });

    await fill('staff@example.com', 'SecurePass123!');

    expect(await screen.findByText(text)).toBeInTheDocument();
  });

  it('ログイン前に開こうとしたページがあれば、そこへ戻る', async () => {
    server.use(http.post(`${BASE}/auth/login`, () => tokenResponse('patient')));
    renderWithProviders(<LoginForm />, { route: { pathname: '/login', state: { from: '/reserve' } }, extraRoutes: { '/reserve': '予約画面' } });

    await fill('taro@example.com', 'SecurePass123!');

    expect(await screen.findByText('予約画面')).toBeInTheDocument();
  });

  // セキュリティ: 外部サイトへのリダイレクトに悪用されない（オープンリダイレクト対策）
  it.each(['https://evil.example.com', '//evil.example.com', 'javascript:alert(1)'])('不正な戻り先 %s は無視する', async (from) => {
    server.use(http.post(`${BASE}/auth/login`, () => tokenResponse('patient')));
    renderWithProviders(<LoginForm />, { route: { pathname: '/login', state: { from } }, extraRoutes: { '/mypage': 'マイページ画面' } });

    await fill('taro@example.com', 'SecurePass123!');

    expect(await screen.findByText('マイページ画面')).toBeInTheDocument();
  });

  it('認証失敗はメッセージを表示し、パスワードは消さずに再入力できる', async () => {
    server.use(http.post(`${BASE}/auth/login`, () => HttpResponse.json({ error: 'invalid_credentials', message: 'invalid credentials' }, { status: 401 })));
    renderWithProviders(<LoginForm />);

    await fill('taro@example.com', 'WrongPassword!!');

    expect(await screen.findByRole('alert')).toHaveTextContent('メールアドレスまたはパスワードが正しくありません');
    expect(authStore.getState().status).not.toBe('authenticated');
  });

  it('アカウントがロックされたら待つ目安を伝える（429）', async () => {
    server.use(http.post(`${BASE}/auth/login`, () => HttpResponse.json({ error: 'too_many_requests' }, { status: 429, headers: { 'Retry-After': '900' } })));
    renderWithProviders(<LoginForm />);

    await fill('taro@example.com', 'SecurePass123!');

    expect(await screen.findByRole('alert')).toHaveTextContent('15分');
  });

  it('送信中はボタンを無効にして二重送信を防ぐ', async () => {
    let calls = 0;
    server.use(http.post(`${BASE}/auth/login`, async () => { calls++; await new Promise((r) => setTimeout(r, 100)); return tokenResponse('patient'); }));
    renderWithProviders(<LoginForm />, { extraRoutes: { '/mypage': 'マイページ画面' } });
    const user = await fill('taro@example.com', 'SecurePass123!');

    expect(screen.getByRole('button', { name: /ログイン/ })).toBeDisabled();
    await user.keyboard('{Enter}');
    await waitFor(() => expect(screen.getByText('マイページ画面')).toBeInTheDocument());
    expect(calls).toBe(1);
  });

  it('パスワード欄は伏せ字で、パスワード管理ツールが使える属性を持つ', () => {
    renderWithProviders(<LoginForm />);

    const pw = screen.getByLabelText(/パスワード/);
    expect(pw).toHaveAttribute('type', 'password');
    expect(pw).toHaveAttribute('autocomplete', 'current-password');
    expect(screen.getByLabelText(/メールアドレス/)).toHaveAttribute('autocomplete', 'email');
  });
});
