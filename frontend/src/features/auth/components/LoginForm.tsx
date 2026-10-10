import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { TextInput } from '@/components/FormField';
import { authStore } from '@/store/auth';
import { useLogin } from '../api/useLogin';
import { landingFor, safeRedirect } from '../redirect';
import { loginSchema, type LoginValues } from '../schemas';

export function LoginForm() {
  const navigate = useNavigate();
  const location = useLocation();
  const login = useLogin();
  const { register, handleSubmit, formState: { errors } } = useForm<LoginValues>({ resolver: zodResolver(loginSchema) });

  const onSubmit = handleSubmit((values) =>
    login.mutate(values, {
      onSuccess: () => {
        const from = safeRedirect((location.state as { from?: unknown } | null)?.from);
        navigate(from ?? landingFor(authStore.getState().role), { replace: true });
      },
    }),
  );

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-4">
      {login.error && <Alert tone="error">{userMessage(parseApiError(login.error))}</Alert>}
      <TextInput label="メールアドレス" type="email" autoComplete="email" required error={errors.email?.message} {...register('email')} />
      <TextInput label="パスワード" type="password" autoComplete="current-password" required error={errors.password?.message} {...register('password')} />
      <Button type="submit" className="w-full" loading={login.isPending}>
        ログイン
      </Button>
      <p className="text-center text-sm">
        <Link to="/password-reset" className="text-primary-700 underline">パスワードをお忘れですか？</Link>
      </p>
    </form>
  );
}
