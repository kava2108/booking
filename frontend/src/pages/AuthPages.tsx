import { Link, Navigate } from 'react-router-dom';
import { Card, PageHeader } from '@/components/Card';
import { PasswordResetConfirmForm } from '@/features/auth/components/PasswordResetConfirmForm';
import { PasswordResetRequestForm } from '@/features/auth/components/PasswordResetRequestForm';
import { LoginForm } from '@/features/auth/components/LoginForm';
import { RegisterForm } from '@/features/auth/components/RegisterForm';
import { landingFor } from '@/features/auth/redirect';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useAuth } from '@/store/auth';

function Narrow({ children }: { children: React.ReactNode }) {
  return <div className="mx-auto max-w-md"><Card>{children}</Card></div>;
}

/** ログイン済みなら、その人の既定の画面へ移動する（ログイン・登録画面は再表示しない）。 */
function useRedirectIfAuthed() {
  const { status, role } = useAuth();
  return status === 'authenticated' ? <Navigate to={landingFor(role)} replace /> : null;
}

export function LoginPage() {
  usePageTitle('ログイン');
  const redirect = useRedirectIfAuthed();
  if (redirect) return redirect;
  return (
    <div className="mx-auto max-w-md">
      <PageHeader title="ログイン" />
      <Card>
        <LoginForm />
      </Card>
      <p className="mt-4 text-center text-sm">
        アカウントをお持ちでない方は <Link to="/register" className="text-primary-700 underline">新規登録</Link>
      </p>
    </div>
  );
}

export function RegisterPage() {
  usePageTitle('新規登録');
  const redirect = useRedirectIfAuthed();
  if (redirect) return redirect;
  return (
    <div className="mx-auto max-w-md">
      <PageHeader title="新規登録" description="ご予約・当日受付・待ち時間のお知らせに必要なアカウントを作成します。" />
      <Card>
        <RegisterForm />
      </Card>
      <p className="mt-4 text-center text-sm">
        登録済みの方は <Link to="/login" className="text-primary-700 underline">ログイン</Link>
      </p>
    </div>
  );
}

export function PasswordResetPage() {
  usePageTitle('パスワードの再設定');
  return (
    <div className="mx-auto max-w-md">
      <PageHeader title="パスワードの再設定" description="登録したメールアドレスを入力してください。再設定のご案内をお送りします。" />
      <Card>
        <PasswordResetRequestForm />
      </Card>
    </div>
  );
}

export function PasswordResetConfirmPage() {
  usePageTitle('新しいパスワードの設定');
  return (
    <div className="mx-auto max-w-md">
      <PageHeader title="新しいパスワードの設定" />
      <Narrow>
        <PasswordResetConfirmForm />
      </Narrow>
    </div>
  );
}
