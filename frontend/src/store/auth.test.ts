import { afterEach, describe, expect, it, vi } from 'vitest';
import { fakeJwt } from '@/test/helpers';
import { authStore } from './auth';

afterEach(() => authStore.reset());

describe('authStore', () => {
  it('JWT から ID とロールを読み取り、認証済みにする', () => {
    authStore.setToken(fakeJwt({ sub: 'abc', role: 'staff' }));

    const s = authStore.getState();
    expect(s.status).toBe('authenticated');
    expect(s.userId).toBe('abc');
    expect(s.role).toBe('staff');
  });

  it('初期状態は loading（セッション復元前）', () => {
    expect(authStore.getState().status).toBe('loading');
  });

  it('clear で匿名になりトークンを破棄する', () => {
    authStore.setToken(fakeJwt());
    authStore.clear();

    expect(authStore.getState()).toMatchObject({ status: 'anonymous', token: null, userId: null, role: null });
    expect(authStore.getToken()).toBeNull();
  });

  it.each([
    ['形式が不正', 'not-a-jwt'],
    ['payload が JSON でない', 'a.%%%.c'],
    ['未知のロール', fakeJwt({ role: 'superuser' })],
    ['旧ロール', fakeJwt({ role: 'user' })],
    ['sub なし', fakeJwt({ sub: '' })],
  ])('不正なトークンは受け入れない: %s', (_name, token) => {
    authStore.setToken(token);
    expect(authStore.getState().status).toBe('anonymous');
    expect(authStore.getToken()).toBeNull();
  });

  it('購読者に変更を通知し、解除できる', () => {
    const fn = vi.fn();
    const off = authStore.subscribe(fn);

    authStore.setToken(fakeJwt());
    expect(fn).toHaveBeenCalledTimes(1);
    off();
    authStore.clear();
    expect(fn).toHaveBeenCalledTimes(1);
  });

  // セキュリティ: アクセストークンはメモリにのみ保持する（XSS で持ち出される場所に置かない）
  it('トークンを Web Storage に保存しない', () => {
    authStore.setToken(fakeJwt());

    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
    expect(document.cookie).toBe('');
  });
});
