import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { PasswordResetConfirmForm } from './PasswordResetConfirmForm';
import { PasswordResetRequestForm } from './PasswordResetRequestForm';

const BASE = '*/api/v1';

describe('PasswordResetRequestForm', () => {
  it('メールアドレスの形式を検証する', async () => {
    renderWithProviders(<PasswordResetRequestForm />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/メールアドレス/), 'bad');
    await user.click(screen.getByRole('button', { name: '再設定メールを送る' }));

    expect(await screen.findByText('メールアドレスの形式が正しくありません')).toBeInTheDocument();
  });

  // REQ-004-AC-2: 登録の有無で表示を変えない（存在の推測防止）
  it('送信後は登録の有無にかかわらず同じ案内を表示する', async () => {
    server.use(http.post(`${BASE}/auth/password-reset/request`, () => HttpResponse.json({ message: 'If the email is registered, a reset link has been sent.' })));
    renderWithProviders(<PasswordResetRequestForm />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/メールアドレス/), 'anyone@example.com');
    await user.click(screen.getByRole('button', { name: '再設定メールを送る' }));

    const status = await screen.findByRole('status');
    expect(status).toHaveTextContent('登録されている場合');
    expect(status).not.toHaveTextContent('anyone@example.com');
  });

  it('レートリミット（429）は待つ目安を伝える', async () => {
    server.use(http.post(`${BASE}/auth/password-reset/request`, () => HttpResponse.json({ error: 'too_many_requests' }, { status: 429, headers: { 'Retry-After': '600' } })));
    renderWithProviders(<PasswordResetRequestForm />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/メールアドレス/), 'a@example.com');
    await user.click(screen.getByRole('button', { name: '再設定メールを送る' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('10分');
  });
});

describe('PasswordResetConfirmForm', () => {
  const renderWithToken = (token: string | null) =>
    renderWithProviders(<PasswordResetConfirmForm />, { route: token === null ? '/password-reset/confirm' : `/password-reset/confirm?token=${token}` });

  it('リンクにトークンが無ければ案内を出し、フォームは表示しない', () => {
    renderWithToken(null);

    expect(screen.getByRole('alert')).toHaveTextContent('リンクが正しくありません');
    expect(screen.queryByLabelText(/新しいパスワード/)).not.toBeInTheDocument();
  });

  it('新しいパスワードは 12 文字以上で、確認と一致が必要', async () => {
    renderWithToken('tok');
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/^新しいパスワード（12/), 'short');
    await user.type(screen.getByLabelText(/新しいパスワード（確認）/), 'short');
    await user.click(screen.getByRole('button', { name: 'パスワードを再設定する' }));

    expect(await screen.findByText('パスワードは12文字以上で入力してください')).toBeInTheDocument();
  });

  it('URL のトークンとともに送信し、成功したらログインへ案内する', async () => {
    let body: unknown;
    server.use(http.post(`${BASE}/auth/password-reset/confirm`, async ({ request }) => { body = await request.json(); return HttpResponse.json({ message: 'password has been reset successfully' }); }));
    renderWithToken('abc-123');
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/^新しいパスワード（12/), 'NewSecurePass456!');
    await user.type(screen.getByLabelText(/新しいパスワード（確認）/), 'NewSecurePass456!');
    await user.click(screen.getByRole('button', { name: 'パスワードを再設定する' }));

    expect(await screen.findByRole('status')).toHaveTextContent('再設定しました');
    expect(screen.getByRole('link', { name: 'ログインへ' })).toHaveAttribute('href', '/login');
    expect(body).toEqual({ token: 'abc-123', new_password: 'NewSecurePass456!' });
  });

  it('期限切れ・使用済みのリンクは再申請を案内する（400）', async () => {
    server.use(http.post(`${BASE}/auth/password-reset/confirm`, () => HttpResponse.json({ error: 'invalid_reset_token' }, { status: 400 })));
    renderWithToken('expired');
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/^新しいパスワード（12/), 'NewSecurePass456!');
    await user.type(screen.getByLabelText(/新しいパスワード（確認）/), 'NewSecurePass456!');
    await user.click(screen.getByRole('button', { name: 'パスワードを再設定する' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('有効期限');
    expect(screen.getByRole('link', { name: 'もう一度申し込む' })).toHaveAttribute('href', '/password-reset');
  });
});
