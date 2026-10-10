import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { TextInput } from '@/components/FormField';
import { usePasswordResetRequest } from '../api/usePasswordReset';
import { resetRequestSchema, type ResetRequestValues } from '../schemas';

export function PasswordResetRequestForm() {
  const request = usePasswordResetRequest();
  const { register, handleSubmit, formState: { errors } } = useForm<ResetRequestValues>({ resolver: zodResolver(resetRequestSchema) });

  // 登録の有無にかかわらず同じ案内を表示する（メールアドレスの存在を推測させない）
  if (request.isSuccess) {
    return (
      <Alert tone="success">
        メールアドレスが登録されている場合、パスワード再設定のご案内を送信しました。メールが届かない場合は、迷惑メールフォルダもご確認ください。
      </Alert>
    );
  }
  return (
    <form onSubmit={handleSubmit((v) => request.mutate(v))} noValidate className="space-y-4">
      {request.error && <Alert tone="error">{userMessage(parseApiError(request.error))}</Alert>}
      <TextInput label="メールアドレス" type="email" autoComplete="email" required error={errors.email?.message} {...register('email')} />
      <Button type="submit" className="w-full" loading={request.isPending}>
        再設定メールを送る
      </Button>
    </form>
  );
}
