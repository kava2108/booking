import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Link, useSearchParams } from 'react-router-dom';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { TextInput } from '@/components/FormField';
import { usePasswordResetConfirm } from '../api/usePasswordReset';
import { resetConfirmSchema, type ResetConfirmValues } from '../schemas';

export function PasswordResetConfirmForm() {
  const [params] = useSearchParams();
  const token = params.get('token');
  const confirm = usePasswordResetConfirm();
  const { register, handleSubmit, formState: { errors } } = useForm<ResetConfirmValues>({ resolver: zodResolver(resetConfirmSchema) });

  if (!token) {
    return (
      <div className="space-y-3">
        <Alert tone="error">リンクが正しくありません。メールに記載されたリンクをそのまま開いてください。</Alert>
        <Link to="/password-reset" className="text-primary-700 underline">もう一度申し込む</Link>
      </div>
    );
  }
  if (confirm.isSuccess) {
    return (
      <div className="space-y-3">
        <Alert tone="success">パスワードを再設定しました。新しいパスワードでログインしてください。</Alert>
        <Link to="/login" className="text-primary-700 underline">ログインへ</Link>
      </div>
    );
  }
  const error = confirm.error ? parseApiError(confirm.error) : null;
  return (
    <form onSubmit={handleSubmit((v) => confirm.mutate({ token, new_password: v.password }))} noValidate className="space-y-4">
      {error && <Alert tone="error">{userMessage(error)}</Alert>}
      {error?.code === 'invalid_reset_token' && <Link to="/password-reset" className="block text-primary-700 underline">もう一度申し込む</Link>}
      <TextInput
        label="新しいパスワード（12文字以上）"
        type="password"
        autoComplete="new-password"
        required
        error={errors.password?.message}
        {...register('password')}
      />
      <TextInput label="新しいパスワード（確認）" type="password" autoComplete="new-password" required error={errors.confirm?.message} {...register('confirm')} />
      <Button type="submit" className="w-full" loading={confirm.isPending}>
        パスワードを再設定する
      </Button>
    </form>
  );
}
