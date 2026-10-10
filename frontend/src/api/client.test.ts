import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { fakeJwt } from '@/test/helpers';
import { server } from '@/test/server';
import { api, restoreSession } from './client';

const BASE = '*/api/v1';

beforeEach(() => authStore.setToken(fakeJwt({ sub: 'old' })));
afterEach(() => authStore.reset());

describe('api client', () => {
  it('アクセストークンを Authorization ヘッダーに付ける', async () => {
    let auth: string | null = null;
    server.use(http.get(`${BASE}/ping`, ({ request }) => { auth = request.headers.get('authorization'); return HttpResponse.json({}); }));

    await api.get('/ping');

    expect(auth).toBe(`Bearer ${authStore.getToken()}`);
  });

  it('未ログインならヘッダーを付けない', async () => {
    authStore.clear();
    let auth: string | null = 'unset';
    server.use(http.get(`${BASE}/ping`, ({ request }) => { auth = request.headers.get('authorization'); return HttpResponse.json({}); }));

    await api.get('/ping');

    expect(auth).toBeNull();
  });

  it('401 のときリフレッシュして元のリクエストを 1 回だけ再試行する', async () => {
    const fresh = fakeJwt({ sub: 'new' });
    const seen: (string | null)[] = [];
    server.use(
      http.get(`${BASE}/me`, ({ request }) => {
        seen.push(request.headers.get('authorization'));
        return request.headers.get('authorization') === `Bearer ${fresh}` ? HttpResponse.json({ ok: true }) : HttpResponse.json({ error: 'unauthorized' }, { status: 401 });
      }),
      http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ access_token: fresh, token_type: 'Bearer', expires_in: 900 })),
    );

    const res = await api.get('/me');

    expect(res.data).toEqual({ ok: true });
    expect(seen).toHaveLength(2);
    expect(authStore.getState().userId).toBe('new');
  });

  it('同時に複数の 401 が起きても、リフレッシュは 1 回だけ（リフレッシュトークンのローテーションを壊さない）', async () => {
    const fresh = fakeJwt({ sub: 'new' });
    let refreshCalls = 0;
    server.use(
      http.get(`${BASE}/a`, ({ request }) => request.headers.get('authorization') === `Bearer ${fresh}` ? HttpResponse.json({}) : HttpResponse.json({}, { status: 401 })),
      http.get(`${BASE}/b`, ({ request }) => request.headers.get('authorization') === `Bearer ${fresh}` ? HttpResponse.json({}) : HttpResponse.json({}, { status: 401 })),
      http.post(`${BASE}/auth/refresh`, async () => { refreshCalls++; await new Promise((r) => setTimeout(r, 30)); return HttpResponse.json({ access_token: fresh, token_type: 'Bearer', expires_in: 900 }); }),
    );

    await Promise.all([api.get('/a'), api.get('/b'), api.get('/a')]);

    expect(refreshCalls).toBe(1);
  });

  it('リフレッシュに失敗したらログアウト状態にして元のエラーを返す', async () => {
    server.use(
      http.get(`${BASE}/me`, () => HttpResponse.json({ error: 'unauthorized' }, { status: 401 })),
      http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ error: 'invalid_token' }, { status: 401 })),
    );

    await expect(api.get('/me')).rejects.toMatchObject({ response: { status: 401 } });

    expect(authStore.getState().status).toBe('anonymous');
    expect(authStore.getToken()).toBeNull();
  });

  it('再試行しても 401 なら無限ループせずエラーにする', async () => {
    let calls = 0;
    server.use(
      http.get(`${BASE}/me`, () => { calls++; return HttpResponse.json({}, { status: 401 }); }),
      http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ access_token: fakeJwt(), token_type: 'Bearer', expires_in: 900 })),
    );

    await expect(api.get('/me')).rejects.toMatchObject({ response: { status: 401 } });
    expect(calls).toBe(2);
  });

  it.each(['/auth/login', '/auth/register', '/auth/password-reset/confirm'])('%s の 401/エラーはリフレッシュしない', async (path) => {
    let refreshed = false;
    server.use(
      http.post(`${BASE}${path}`, () => HttpResponse.json({ error: 'invalid_credentials' }, { status: 401 })),
      http.post(`${BASE}/auth/refresh`, () => { refreshed = true; return HttpResponse.json({}); }),
    );

    await expect(api.post(path, {})).rejects.toMatchObject({ response: { status: 401 } });
    expect(refreshed).toBe(false);
  });

  it('未ログインの 401 ではリフレッシュを試みない（ログイン前の保護 API）', async () => {
    authStore.clear();
    let refreshed = false;
    server.use(
      http.get(`${BASE}/me`, () => HttpResponse.json({}, { status: 401 })),
      http.post(`${BASE}/auth/refresh`, () => { refreshed = true; return HttpResponse.json({}); }),
    );

    await expect(api.get('/me')).rejects.toMatchObject({ response: { status: 401 } });
    expect(refreshed).toBe(false);
  });

  it('401 以外のエラーはそのまま返す', async () => {
    server.use(http.get(`${BASE}/x`, () => HttpResponse.json({ error: 'forbidden' }, { status: 403 })));

    await expect(api.get('/x')).rejects.toMatchObject({ response: { status: 403 } });
    expect(authStore.getState().status).toBe('authenticated');
  });
});

describe('restoreSession（起動時のセッション復元）', () => {
  beforeEach(() => authStore.reset());

  it('リフレッシュ用 Cookie があれば認証済みにする', async () => {
    const token = fakeJwt({ role: 'staff' });
    server.use(http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ access_token: token, token_type: 'Bearer', expires_in: 900 })));

    await restoreSession();

    expect(authStore.getState()).toMatchObject({ status: 'authenticated', role: 'staff' });
  });

  it('Cookie が無い/無効なら匿名になる（エラーにしない）', async () => {
    server.use(http.post(`${BASE}/auth/refresh`, () => HttpResponse.json({ error: 'invalid_token' }, { status: 401 })));

    await expect(restoreSession()).resolves.toBeUndefined();

    expect(authStore.getState().status).toBe('anonymous');
  });

  it('通信失敗でも匿名になる', async () => {
    server.use(http.post(`${BASE}/auth/refresh`, () => HttpResponse.error()));

    await restoreSession();

    expect(authStore.getState().status).toBe('anonymous');
  });
});
