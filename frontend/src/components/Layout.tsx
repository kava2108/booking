import { Stethoscope } from 'lucide-react';
import { Link, NavLink, Outlet } from 'react-router-dom';
import { useLogout } from '@/features/auth/api/useLogout';
import { cn } from '@/lib/utils';
import { isAdminRole, isStaffRole, useAuth } from '@/store/auth';
import { Button } from './Button';

const link = ({ isActive }: { isActive: boolean }) =>
  cn('inline-flex min-h-11 items-center rounded-lg px-3 text-sm font-medium', isActive ? 'bg-primary-100 text-primary-800' : 'text-neutral-700 hover:bg-neutral-100');

export function AppLayout() {
  const { status, role } = useAuth();
  const logout = useLogout();
  const authed = status === 'authenticated';

  return (
    <div className="flex min-h-screen flex-col">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:rounded focus:bg-white focus:p-2">
        本文へスキップ
      </a>
      <header className="border-b border-neutral-200 bg-white">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-2 px-4 py-2">
          <Link to="/" className="flex items-center gap-2 text-lg font-bold text-primary-700">
            <Stethoscope className="h-6 w-6" aria-hidden="true" />
            クリニック予約
          </Link>
          <nav aria-label="メインメニュー" className="flex flex-wrap items-center gap-1">
            <NavLink to="/" end className={link}>待ち時間</NavLink>
            <NavLink to="/reserve" className={link}>予約する</NavLink>
            {authed && <NavLink to="/mypage" className={link}>マイページ</NavLink>}
            {authed && isStaffRole(role) && <NavLink to="/staff" className={link}>スタッフ</NavLink>}
            {authed && isAdminRole(role) && <NavLink to="/admin" className={link}>管理</NavLink>}
            {status === 'anonymous' && (
              <>
                <NavLink to="/login" className={link}>ログイン</NavLink>
                <NavLink to="/register" className={link}>新規登録</NavLink>
              </>
            )}
            {authed && (
              <Button variant="ghost" size="sm" onClick={() => logout.mutate()} loading={logout.isPending}>
                ログアウト
              </Button>
            )}
          </nav>
        </div>
      </header>
      <main id="main" className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">
        <Outlet />
      </main>
      <footer className="border-t border-neutral-200 bg-white py-4 text-center text-xs text-neutral-600">
        <p>待ち時間は目安です。急を要する症状の場合は、お電話または救急にご連絡ください。</p>
      </footer>
    </div>
  );
}
