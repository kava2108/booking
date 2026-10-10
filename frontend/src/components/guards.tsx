import { Navigate, Outlet, useLocation } from 'react-router-dom';
import type { Role } from '@/api/types';
import { useAuth } from '@/store/auth';
import { Alert } from './Alert';
import { Spinner } from './Spinner';

/**
 * 画面の出し分け用ガード。ここで弾くのは利便性のためで、認可そのものではない。
 * 本当の認可は API（サーバー）が JWT のロールで強制する。
 */

/** ログイン必須。セッション復元中は待ち、未ログインならログイン画面へ（戻り先を保持）。 */
export function RequireAuth() {
  const { status } = useAuth();
  const location = useLocation();
  if (status === 'loading') return <Spinner />;
  if (status === 'anonymous') return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />;
  return <Outlet />;
}

/** 指定ロールのみ。ログイン済みでロールが違う場合は、権限なしの案内を表示する。 */
export function RequireRole({ roles }: { roles: readonly Role[] }) {
  const { status, role } = useAuth();
  const location = useLocation();
  if (status === 'loading') return <Spinner />;
  if (status === 'anonymous') return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />;
  if (!role || !roles.includes(role)) {
    return <Alert tone="error">この画面を利用する権限がありません。</Alert>;
  }
  return <Outlet />;
}
