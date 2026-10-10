import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, type RenderOptions } from '@testing-library/react';
import { createElement, type ReactElement, type ReactNode } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { authStore } from '@/store/auth';

const b64url = (o: unknown) => btoa(JSON.stringify(o)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_');

/** テスト用の JWT（署名は検証しない。クライアントは表示用に payload を読むだけ）。 */
export function fakeJwt(payload: { sub?: string; role?: string; exp?: number } = {}): string {
  const exp = payload.exp ?? Math.floor(Date.now() / 1000) + 900;
  return `${b64url({ alg: 'HS256', typ: 'JWT' })}.${b64url({ sub: 'user-1', role: 'patient', ...payload, exp })}.sig`;
}

export function loginAs(role: string = 'patient', sub = 'user-1'): string {
  const token = fakeJwt({ role, sub });
  authStore.setToken(token);
  return token;
}

export function newQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
}

interface Options extends Omit<RenderOptions, 'wrapper'> {
  route?: string | { pathname: string; state?: unknown };
  /** 追加のルート（遷移先の確認用）。path → 表示する文字列 */
  extraRoutes?: Record<string, string>;
  path?: string;
}

/** Router と QueryClient を付けて描画する。 */
export function renderWithProviders(ui: ReactElement, { route = '/', path = '*', extraRoutes = {}, ...opts }: Options = {}) {
  const client = newQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(
      QueryClientProvider,
      { client },
      createElement(
        MemoryRouter,
        { initialEntries: [route] },
        createElement(
          Routes,
          null,
          createElement(Route, { path, element: children }),
          ...Object.entries(extraRoutes).map(([p, text]) => createElement(Route, { key: p, path: p, element: createElement('div', null, text) })),
        ),
      ),
    );
  return { client, ...render(ui, { wrapper, ...opts }) };
}

// ---- API 応答のテストデータ ----
export const waitTimes = (departments: unknown[], updatedAt = '2026-10-12T09:41:10+09:00') => ({
  updated_at: updatedAt,
  note: '待ち時間は目安です。混雑状況により変動します',
  departments,
});

export const available = (over: Record<string, unknown> = {}) => ({
  department_id: 1, name: '内科', status: 'available', waiting_count: 6, wait_minutes: 35, basis: 'actual', ...over,
});

export const unavailable = (reason: string, over: Record<string, unknown> = {}) => ({
  department_id: 2, name: '小児科', status: 'unavailable', reason, ...over,
});

export const dept = (id = 1, name = '内科') => ({ id, name });
export const doctor = (id = 7, name = '山田', departmentId = 1) => ({ id, department_id: departmentId, name });
export const slot = (id: string, startAt: string, over: Record<string, unknown> = {}) => ({
  id, doctor_id: 7, start_at: startAt, available: true, ...over,
});
export const appt = (id: string, over: Record<string, unknown> = {}) => ({
  id, appointment_no: 'A-20261019-0042', slot_start: '2026-10-19T09:30:00+09:00', department: '内科', doctor: '山田',
  visit_type: 'first', status: 'booked', cancelable: true, symptom_note: '咳', ...over,
});

export const staffAppt = (id: string, over: Record<string, unknown> = {}) => ({
  id, appointment_no: 'A-1', slot_start: '2026-10-12T09:30:00+09:00', doctor: '山田', patient_name: '山田 太郎',
  visit_type: 'first', status: 'booked', checked_in: false, ...over,
});
export const queueEntry = (id: string, over: Record<string, unknown> = {}) => ({
  id, ticket_no: 1, source: 'reservation', status: 'waiting', patient_name: '山田 太郎', symptom_note: '咳', checked_in_at: '2026-10-12T09:00:00+09:00', ...over,
});
