import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { fakeJwt, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { RegisterForm } from './RegisterForm';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

async function submit(v: { name?: string; email?: string; password?: string; confirm?: string }) {
  const user = userEvent.setup();
  if (v.name) await user.type(screen.getByLabelText(/お名前/), v.name);
  if (v.email) await user.type(screen.getByLabelText(/メールアドレス/), v.email);
  if (v.password) await user.type(screen.getByLabelText(/^パスワード（12/), v.password);
  if (v.confirm) await user.type(screen.getByLabelText(/パスワード（確認）/), v.confirm);
  await user.click(screen.getByRole('button', { name: '登録する' }));
}

describe('RegisterForm', () => {
  it('必須項目が未入力ならエラー', async () => {
    renderWithProviders(<RegisterForm />);

    await submit({});

    expect(await screen.findByText('お名前を入力してください')).toBeInTheDocument();
    expect(screen.getByText('メールアドレスを入力してください')).toBeInTheDocument();
  });

  // REQ-001-AC-3: パスワードは 12 文字以上
  it('パスワードが 11 文字なら拒否し、12 文字なら受け付ける', async () => {
    let called = false;
    server.use(
      http.post(`${BASE}/auth/register`, () => { called = true; return HttpResponse.json({ user_id: 'u', email: 'a@example.com', name: 'n', created_at: new Date().toISOString() }, { status: 201 }); }),
      http.post(`${BASE}/auth/login`, () => HttpResponse.json({ access_token: fakeJwt(), token_type: 'Bearer', expires_in: 900 })),
    );
    renderWithProviders(<RegisterForm />, { extraRoutes: { '/mypage': 'マイページ画面' } });

    await submit({ name: '山田', email: 'a@example.com', password: 'abcdefghijk', confirm: 'abcdefghijk' });
    expect(await screen.findByText('パスワードは12文字以上で入力してください')).toBeInTheDocument();
    expect(called).toBe(false);

    const user = userEvent.setup();
    await user.clear(screen.getByLabelText(/^パスワード（12/));
    await user.clear(screen.getByLabelText(/パスワード（確認）/));
    await user.type(screen.getByLabelText(/^パスワード（12/), 'abcdefghijkl');
    await user.type(screen.getByLabelText(/パスワード（確認）/), 'abcdefghijkl');
    await user.click(screen.getByRole('button', { name: '登録する' }));

    expect(await screen.findByText('マイページ画面')).toBeInTheDocument();
    expect(called).toBe(true);
  });

  it('日本語のパスワードは文字数で数える（12 文字）が、72 バイトを超えると拒否する', async () => {
    renderWithProviders(<RegisterForm />);

    const long = 'あ'.repeat(25); // 25 文字 = 75 バイト
    await submit({ name: '山田', email: 'a@example.com', password: long, confirm: long });

    expect(await screen.findByText('パスワードが長すぎます')).toBeInTheDocument();
  });

  it('確認用パスワードが一致しなければエラー', async () => {
    renderWithProviders(<RegisterForm />);

    await submit({ name: '山田', email: 'a@example.com', password: 'abcdefghijkl', confirm: 'abcdefghijkx' });

    expect(await screen.findByText('パスワードが一致しません')).toBeInTheDocument();
  });

  it('登録後は自動でログインしてマイページへ進む', async () => {
    const calls: string[] = [];
    server.use(
      http.post(`${BASE}/auth/register`, async ({ request }) => {
        calls.push('register');
        const body = (await request.json()) as Record<string, unknown>;
        expect(body).toEqual({ name: '山田 太郎', email: 'taro@example.com', password: 'SecurePass123!' });
        return HttpResponse.json({ user_id: 'u', email: 'taro@example.com', name: '山田 太郎', created_at: new Date().toISOString() }, { status: 201 });
      }),
      http.post(`${BASE}/auth/login`, () => { calls.push('login'); return HttpResponse.json({ access_token: fakeJwt(), token_type: 'Bearer', expires_in: 900 }); }),
    );
    renderWithProviders(<RegisterForm />, { extraRoutes: { '/mypage': 'マイページ画面' } });

    await submit({ name: '山田 太郎', email: 'taro@example.com', password: 'SecurePass123!', confirm: 'SecurePass123!' });

    expect(await screen.findByText('マイページ画面')).toBeInTheDocument();
    expect(calls).toEqual(['register', 'login']);
    expect(authStore.getState().status).toBe('authenticated');
  });

  it('登録済みのメールアドレスは案内を表示する（409）', async () => {
    server.use(http.post(`${BASE}/auth/register`, () => HttpResponse.json({ error: 'email_already_registered' }, { status: 409 })));
    renderWithProviders(<RegisterForm />);

    await submit({ name: '山田', email: 'dup@example.com', password: 'abcdefghijkl', confirm: 'abcdefghijkl' });

    expect(await screen.findByRole('alert')).toHaveTextContent('登録済み');
  });

  it('サーバー側のフィールドエラー（422）を該当欄に表示する', async () => {
    server.use(http.post(`${BASE}/auth/register`, () => HttpResponse.json({ error: 'validation_failed', details: [{ field: 'email', message: 'must be a valid email address' }] }, { status: 422 })));
    renderWithProviders(<RegisterForm />);

    await submit({ name: '山田', email: 'a@example.com', password: 'abcdefghijkl', confirm: 'abcdefghijkl' });

    expect(await screen.findByText('メールアドレスの形式が正しくありません')).toBeInTheDocument();
  });

  it('新規パスワード欄は new-password、伏せ字', () => {
    renderWithProviders(<RegisterForm />);

    const pw = screen.getByLabelText(/^パスワード（12/);
    expect(pw).toHaveAttribute('type', 'password');
    expect(pw).toHaveAttribute('autocomplete', 'new-password');
  });
});
