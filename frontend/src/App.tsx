import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { restoreSession } from '@/api/client';
import { ApiError, parseApiError } from '@/api/errors';
import { RequireAuth, RequireRole } from '@/components/guards';
import { AppLayout } from '@/components/Layout';
import { AdminPage } from '@/pages/AdminPage';
import { LoginPage, PasswordResetConfirmPage, PasswordResetPage, RegisterPage } from '@/pages/AuthPages';
import { HomePage } from '@/pages/HomePage';
import { MyPage } from '@/pages/MyPage';
import { NotFoundPage } from '@/pages/NotFoundPage';
import { ReservePage } from '@/pages/ReservePage';
import { StaffPage } from '@/pages/StaffPage';

/** 画面の対応。ロールによる出し分けは利便性のためで、認可の本体は API（サーバー）が行う。 */
export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<HomePage />} />
        <Route path="reserve" element={<ReservePage />} />
        <Route path="login" element={<LoginPage />} />
        <Route path="register" element={<RegisterPage />} />
        <Route path="password-reset" element={<PasswordResetPage />} />
        <Route path="password-reset/confirm" element={<PasswordResetConfirmPage />} />
        <Route element={<RequireAuth />}>
          <Route path="mypage" element={<MyPage />} />
        </Route>
        <Route element={<RequireRole roles={['staff', 'clinic_admin']} />}>
          <Route path="staff" element={<StaffPage />} />
        </Route>
        <Route element={<RequireRole roles={['clinic_admin']} />}>
          <Route path="admin" element={<AdminPage />} />
        </Route>
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}

/** 4xx（入力・権限・競合）は再試行しても結果が変わらないので再試行しない。通信失敗や 5xx だけ少し再試行する。 */
function shouldRetry(count: number, error: unknown): boolean {
  const e: ApiError = parseApiError(error);
  return (e.status === 0 || e.status >= 500) && count < 2;
}

export function App() {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: shouldRetry, refetchOnWindowFocus: true }, mutations: { retry: false } } }));

  // 起動時に、HttpOnly Cookie のリフレッシュトークンからログイン状態を復元する（無ければ未ログイン）
  useEffect(() => {
    void restoreSession();
  }, []);

  return (
    <QueryClientProvider client={client}>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </QueryClientProvider>
  );
}
