import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { TextInput } from '@/components/FormField';
import { useStaffWalkIn } from '../api/useStaff';

const schema = z.object({
  name: z
    .string()
    .trim()
    .min(1, '氏名を入力してください')
    .refine((v) => [...v].length <= 100, '100文字以内で入力してください'),
});
type Values = z.infer<typeof schema>;

/** 当日受付の代行。電話・窓口で来たアカウントのない患者を、氏名だけで待機列に追加する。 */
export function StaffWalkInForm({ departmentId }: { departmentId: number }) {
  const walkIn = useStaffWalkIn();
  const { register, handleSubmit, reset, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { name: '' } });

  const onSubmit = handleSubmit((v) => walkIn.mutate({ departmentId, displayName: v.name }, { onSuccess: () => reset() }));

  return (
    <div className="max-w-md space-y-4">
      {walkIn.error && <Alert tone="error">{userMessage(parseApiError(walkIn.error))}</Alert>}
      {walkIn.isSuccess && (
        <Alert tone="success">
          <strong>受付番号 {walkIn.data.ticket_no}</strong>
          <span className="ml-2">{walkIn.data.ahead_count > 0 ? `あと${walkIn.data.ahead_count}人でお呼びします` : 'まもなくお呼びします'}</span>
        </Alert>
      )}
      <form onSubmit={onSubmit} noValidate className="space-y-4">
        <TextInput label="氏名" autoComplete="off" hint="アカウントをお持ちでない方の受付です。氏名は待機列でのみ表示されます。" error={errors.name?.message} {...register('name')} />
        <Button type="submit" variant="cta" loading={walkIn.isPending}>受付する</Button>
      </form>
    </div>
  );
}
