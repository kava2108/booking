import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Link } from 'react-router-dom';
import { z } from 'zod';
import type { Slot } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Dialog } from '@/components/Dialog';
import { TextArea } from '@/components/FormField';
import { Spinner } from '@/components/Spinner';
import { formatDateLong, formatTime } from '@/lib/format';
import { useAuth } from '@/store/auth';
import { useCreateAppointment } from '../api/useAppointments';

const schema = z.object({
  visit_type: z.enum(['first', 'revisit'], { error: '初診か再診かを選んでください' }),
  symptom_note: z.string().refine((v) => [...v].length <= 1000, '1000文字以内で入力してください'),
});
type Values = z.infer<typeof schema>;

interface Props {
  slot: Slot;
  departmentName: string;
  doctorName: string;
  onClose: () => void;
}

/** 予約の確認と確定。ログインが必要。満席（409）なら別の枠を選び直せる。 */
export function BookingDialog({ slot, departmentName, doctorName, onClose }: Props) {
  const { status } = useAuth();
  const create = useCreateAppointment();
  const { register, handleSubmit, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { symptom_note: '' } });
  const error = create.error ? parseApiError(create.error) : null;

  return (
    <Dialog title="予約内容の確認" onClose={onClose}>
      <dl className="mb-4 grid grid-cols-[6rem_1fr] gap-y-1 text-sm">
        <dt className="text-neutral-600">診療科</dt><dd>{departmentName}</dd>
        <dt className="text-neutral-600">担当医</dt><dd>{doctorName}</dd>
        <dt className="text-neutral-600">日時</dt><dd>{formatDateLong(slot.start_at)} {formatTime(slot.start_at)}</dd>
      </dl>

      {status === 'loading' && <Spinner />}
      {status === 'anonymous' && (
        <div className="space-y-4">
          <p>ご予約にはログインが必要です。アカウントをお持ちでない場合は新規登録してください。</p>
          <div className="flex flex-wrap gap-2">
            <Link to="/login" state={{ from: '/reserve' }} className="inline-flex min-h-11 items-center rounded-lg bg-primary-500 px-4 font-medium text-white hover:bg-primary-600">ログインする</Link>
            <Link to="/register" className="inline-flex min-h-11 items-center rounded-lg border border-primary-300 px-4 font-medium text-primary-700">新規登録</Link>
          </div>
        </div>
      )}

      {status === 'authenticated' && create.isSuccess && (
        <div className="space-y-4">
          <Alert tone="success"><strong>予約を受け付けました</strong></Alert>
          <p className="text-sm text-neutral-600">予約番号</p>
          <strong className="block text-2xl text-primary-700">{create.data.appointment_no}</strong>
          <p className="text-sm text-neutral-600">確認のメールをお送りします。当日は受付でお名前をお伝えください。</p>
          <div className="flex flex-wrap justify-end gap-2">
            <Button variant="ghost" onClick={onClose}>閉じる</Button>
            <Link to="/mypage" className="inline-flex min-h-11 items-center rounded-lg bg-primary-500 px-4 font-medium text-white hover:bg-primary-600">マイページで確認する</Link>
          </div>
        </div>
      )}

      {status === 'authenticated' && !create.isSuccess && (
        <form
          onSubmit={handleSubmit((v) => create.mutate({ slot_id: slot.id, visit_type: v.visit_type, symptom_note: v.symptom_note }))}
          noValidate
          className="space-y-4"
        >
          {error && <Alert tone="error">{userMessage(error)}</Alert>}
          <fieldset>
            <legend className="mb-1 text-sm font-medium">来院区分<span className="ml-1 text-red-600" aria-hidden="true">*</span></legend>
            <div className="flex gap-4">
              <label className="flex min-h-11 items-center gap-2"><input type="radio" value="first" {...register('visit_type')} />初診</label>
              <label className="flex min-h-11 items-center gap-2"><input type="radio" value="revisit" {...register('visit_type')} />再診</label>
            </div>
            {errors.visit_type && <p role="alert" className="text-sm text-red-700">{errors.visit_type.message}</p>}
          </fieldset>
          <TextArea
            label="症状など（任意）"
            hint="入力内容は、あなたと担当スタッフだけが閲覧できます。メールには記載されません。"
            error={errors.symptom_note?.message}
            {...register('symptom_note')}
          />
          <div className="flex flex-wrap justify-end gap-2">
            {error?.code === 'slot_full' ? (
              <Button variant="secondary" onClick={onClose}>別の枠を選ぶ</Button>
            ) : (
              <Button variant="ghost" onClick={onClose}>戻る</Button>
            )}
            <Button type="submit" variant="cta" loading={create.isPending}>予約を確定する</Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}
