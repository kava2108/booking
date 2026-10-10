import { useSyncExternalStore } from 'react';
import type { Role } from '@/api/types';

// アクセストークンはメモリにのみ保持する（localStorage 等に置かない = XSS で持ち出されにくくする）。
// リフレッシュトークンは HttpOnly Cookie でブラウザが管理し、JS からは読めない。
// JWT の payload は表示（ロールによる画面の出し分け）のために読むだけで、認可の根拠にしない。認可は常にサーバーが行う。

export interface AuthState {
  /** loading = セッション復元前。anonymous = 未ログイン。 */
  status: 'loading' | 'authenticated' | 'anonymous';
  token: string | null;
  userId: string | null;
  role: Role | null;
}

const ROLES: readonly string[] = ['patient', 'staff', 'clinic_admin', 'system_admin'];
const LOADING: AuthState = { status: 'loading', token: null, userId: null, role: null };
const ANONYMOUS: AuthState = { status: 'anonymous', token: null, userId: null, role: null };

let state: AuthState = LOADING;
const listeners = new Set<() => void>();

function set(next: AuthState): void {
  state = next;
  listeners.forEach((l) => l());
}

function decode(token: string): { userId: string; role: Role } | null {
  try {
    const part = token.split('.')[1];
    if (!part) return null;
    const json = atob(part.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(part.length / 4) * 4, '='));
    const p = JSON.parse(json) as { sub?: unknown; role?: unknown };
    if (typeof p.sub !== 'string' || p.sub === '' || typeof p.role !== 'string' || !ROLES.includes(p.role)) return null;
    return { userId: p.sub, role: p.role as Role };
  } catch {
    return null;
  }
}

export const authStore = {
  getState: (): AuthState => state,
  getToken: (): string | null => state.token,
  /** 不正なトークンは受け入れず、匿名にする。 */
  setToken(token: string): void {
    const d = decode(token);
    if (!d) return set(ANONYMOUS);
    set({ status: 'authenticated', token, userId: d.userId, role: d.role });
  },
  clear: (): void => set(ANONYMOUS),
  /** テスト用: 起動直後（セッション復元前）の状態に戻す。 */
  reset: (): void => set(LOADING),
  subscribe(fn: () => void): () => void {
    listeners.add(fn);
    return () => listeners.delete(fn);
  },
};

export function useAuth(): AuthState {
  return useSyncExternalStore(authStore.subscribe, authStore.getState, authStore.getState);
}

export const isStaffRole = (r: Role | null): boolean => r === 'staff' || r === 'clinic_admin';
export const isAdminRole = (r: Role | null): boolean => r === 'clinic_admin';
