import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';
import { authStore } from '@/store/auth';
import type { TokenResponse } from './types';

// 同一オリジンの /api/v1 に接続する（リフレッシュトークンは HttpOnly + SameSite=Strict の Cookie）。
const baseURL = '/api/v1';

/** 通常の API 呼び出し。アクセストークンの付与と、401 時の自動更新 + 再試行を行う。 */
export const api = axios.create({ baseURL, withCredentials: true });
/** インターセプターなし。リフレッシュ自体の 401 で再帰しないために分ける。 */
const bare = axios.create({ baseURL, withCredentials: true });

// 認証系のエンドポイントの 401 は「認証失敗」そのもの。リフレッシュして再試行してはいけない。
const AUTH_PATHS = ['/auth/login', '/auth/register', '/auth/refresh', '/auth/password-reset/'];
const isAuthPath = (url?: string): boolean => !!url && AUTH_PATHS.some((p) => url.startsWith(p));

let refreshing: Promise<string | null> | null = null;

/**
 * アクセストークンを更新する。リフレッシュトークンはローテーションされ 1 回しか使えないため、
 * 同時に複数の 401 が起きても更新は 1 回にまとめる。失敗したらログアウト状態にする。
 */
export function refreshAccessToken(): Promise<string | null> {
  refreshing ??= bare
    .post<TokenResponse>('/auth/refresh')
    .then((r) => {
      authStore.setToken(r.data.access_token);
      return authStore.getToken();
    })
    .catch(() => {
      authStore.clear();
      return null;
    })
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

/** 起動時に呼ぶ。リフレッシュ用 Cookie が有効ならログイン状態を復元し、無ければ匿名にする。 */
export async function restoreSession(): Promise<void> {
  await refreshAccessToken();
}

api.interceptors.request.use((cfg) => {
  const token = authStore.getToken();
  if (token) cfg.headers.set('Authorization', `Bearer ${token}`);
  return cfg;
});

type Retriable = InternalAxiosRequestConfig & { _retried?: boolean };

api.interceptors.response.use(
  (r) => r,
  async (error: AxiosError) => {
    const cfg = error.config as Retriable | undefined;
    const current = authStore.getToken();
    // ログイン中の保護 API が 401 のときだけ、1 回だけ更新して再試行する
    if (error.response?.status !== 401 || !cfg || cfg._retried || isAuthPath(cfg.url) || current === null) throw error;
    cfg._retried = true;

    // 送信後に別のリクエストが既に更新済みなら、新しいトークンで再試行するだけでよい
    const sent = String(cfg.headers.get('Authorization') ?? '');
    const token = sent === `Bearer ${current}` ? await refreshAccessToken() : current;
    if (!token) throw error;
    cfg.headers.set('Authorization', `Bearer ${token}`);
    return api.request(cfg);
  },
);
