import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { AppLayout } from './Layout';

afterEach(() => authStore.reset());

const nav = () => screen.getByRole('navigation', { name: 'メインメニュー' });
const has = (name: string) => !!nav().querySelector(`a[href="${name}"]`);

describe('AppLayout', () => {
  it('未ログイン: 待ち時間・予約・ログイン・新規登録のみ', () => {
    authStore.clear();
    renderWithProviders(<AppLayout />);

    expect(has('/')).toBe(true);
    expect(has('/reserve')).toBe(true);
    expect(has('/login')).toBe(true);
    expect(has('/register')).toBe(true);
    expect(has('/mypage')).toBe(false);
    expect(has('/staff')).toBe(false);
    expect(has('/admin')).toBe(false);
    expect(screen.queryByRole('button', { name: 'ログアウト' })).not.toBeInTheDocument();
  });

  it('患者: マイページとログアウトが出る。スタッフ・管理は出ない', () => {
    loginAs('patient');
    renderWithProviders(<AppLayout />);

    expect(has('/mypage')).toBe(true);
    expect(has('/staff')).toBe(false);
    expect(has('/admin')).toBe(false);
    expect(has('/login')).toBe(false);
    expect(screen.getByRole('button', { name: 'ログアウト' })).toBeInTheDocument();
  });

  it('スタッフ: スタッフ画面が出る。管理は出ない', () => {
    loginAs('staff');
    renderWithProviders(<AppLayout />);

    expect(has('/staff')).toBe(true);
    expect(has('/admin')).toBe(false);
  });

  it('クリニック管理者: スタッフ画面と管理画面が出る', () => {
    loginAs('clinic_admin');
    renderWithProviders(<AppLayout />);

    expect(has('/staff')).toBe(true);
    expect(has('/admin')).toBe(true);
  });

  it('ログアウトするとサーバーに通知し、端末側の状態を破棄する', async () => {
    loginAs('patient');
    let called = false;
    server.use(http.post('*/api/v1/auth/logout', () => { called = true; return HttpResponse.json({ message: 'logged out' }); }));
    const { client } = renderWithProviders(<AppLayout />);
    client.setQueryData(['secret'], { name: '山田' });

    await userEvent.setup().click(screen.getByRole('button', { name: 'ログアウト' }));

    await waitFor(() => expect(authStore.getState().status).toBe('anonymous'));
    expect(called).toBe(true);
    expect(client.getQueryData(['secret'])).toBeUndefined();
  });

  it('サーバーのログアウトが失敗しても端末側はログアウトする（共有端末対策）', async () => {
    loginAs('patient');
    server.use(http.post('*/api/v1/auth/logout', () => HttpResponse.error()));
    renderWithProviders(<AppLayout />);

    await userEvent.setup().click(screen.getByRole('button', { name: 'ログアウト' }));

    await waitFor(() => expect(authStore.getState().status).toBe('anonymous'));
  });

  it('ランドマークとスキップリンクがある（アクセシビリティ）', () => {
    authStore.clear();
    renderWithProviders(<AppLayout />);

    expect(screen.getByRole('banner')).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '本文へスキップ' })).toHaveAttribute('href', '#main');
  });
});
