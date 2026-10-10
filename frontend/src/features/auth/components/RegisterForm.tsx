import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { TextInput } from '@/components/FormField';
import { useLogin } from '../api/useLogin';
import { useRegister } from '../api/useRegister';
import { serverFieldMessage, registerSchema, type RegisterValues } from '../schemas';

export function RegisterForm() {
  const navigate = useNavigate();
  const registerUser = useRegister();
  const login = useLogin();
  const { register, handleSubmit, setError, formState: { errors } } = useForm<RegisterValues>({ resolver: zodResolver(registerSchema) });

  const onSubmit = handleSubmit(async ({ name, email, password }) => {
    try {
      await registerUser.mutateAsync({ name, email, password });
    } catch (err) {
      // サーバー側の検証エラーは該当欄に表示する
      const e = parseApiError(err);
      for (const [field, msg] of Object.entries(e.fields)) {
        if (field === 'email' || field === 'password' || field === 'name') setError(field, { message: serverFieldMessage(msg) });
      }
      return;
    }
    // 登録できたら、そのままログインして利用開始できるようにする
    await login.mutateAsync({ email, password }).then(() => navigate('/mypage', { replace: true })).catch(() => navigate('/login', { replace: true }));
  });

  const error = registerUser.error ? parseApiError(registerUser.error) : null;
  const showAlert = error && Object.keys(error.fields).length === 0;

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-4">
      {showAlert && <Alert tone="error">{userMessage(error)}</Alert>}
      <TextInput label="お名前" autoComplete="name" required error={errors.name?.message} {...register('name')} />
      <TextInput label="メールアドレス" type="email" autoComplete="email" required error={errors.email?.message} {...register('email')} />
      <TextInput
        label="パスワード（12文字以上）"
        type="password"
        autoComplete="new-password"
        required
        hint="12文字以上で、他のサービスと別のものをご利用ください。"
        error={errors.password?.message}
        {...register('password')}
      />
      <TextInput label="パスワード（確認）" type="password" autoComplete="new-password" required error={errors.confirm?.message} {...register('confirm')} />
      <Button type="submit" className="w-full" loading={registerUser.isPending || login.isPending}>
        登録する
      </Button>
    </form>
  );
}
