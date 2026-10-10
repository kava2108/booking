import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import type { ClosureRequest } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Dialog } from '@/components/Dialog';
import { SelectInput, TextArea, TextInput } from '@/components/FormField';
import { useDoctors } from '@/features/catalog/api/useCatalog';
import { useCreateClosure } from '../api/useStaff';

const schema = z
  .object({
    doctor: z.string(),
    from: z.string().min(1, '開始日時を入力してください'),
    to: z.string().min(1, '終了日時を入力してください'),
    reason: z.string().refine((v) => [...v].length <= 200, '200文字以内で入力してください'),
  })
  .refine((v) => !v.from || !v.to || v.to > v.from, { path: ['to'], message: '終了は開始より後にしてください' });
type Values = z.infer<typeof schema>;

/** <input type="datetime-local"> の値（"2026-10-12T09:00"）を日本時間のオフセット付き ISO にする。 */
const toJst = (local: string): string => `${local.length === 16 ? `${local}:00` : local}+09:00`;

/** 休診・医師不在の設定。確定の前に影響（枠が閉じる・予約者へ通知）を説明して確認する。 */
export function ClosureForm({ departmentId }: { departmentId: number }) {
  const doctors = useDoctors(departmentId);
  const create = useCreateClosure();
  const [pending, setPending] = useState<ClosureRequest | null>(null);
  const { register, handleSubmit, reset, formState: { errors } } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { doctor: '', from: '', to: '', reason: '' },
  });

  const onSubmit = handleSubmit((v) => {
    create.reset();
    setPending({
      ...(v.doctor ? { doctor_id: Number(v.doctor) } : {}),
      from_at: toJst(v.from),
      to_at: toJst(v.to),
      reason: v.reason,
    });
  });

  const confirm = () => pending && create.mutate(pending, { onSuccess: () => { setPending(null); reset(); } });

  return (
    <div className="max-w-xl space-y-4">
      {create.isSuccess && (
        <Alert tone="success">{create.data.closed_slots}枠を休診にし、{create.data.notified}人に通知しました。</Alert>
      )}
      <form onSubmit={onSubmit} noValidate className="space-y-4">
        <SelectInput label="対象" {...register('doctor')}>
          <option value="">医療機関全体</option>
          {doctors.data?.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </SelectInput>
        <div className="grid gap-4 sm:grid-cols-2">
          <TextInput label="開始" type="datetime-local" error={errors.from?.message} {...register('from')} />
          <TextInput label="終了" type="datetime-local" error={errors.to?.message} {...register('to')} />
        </div>
        <TextArea label="理由（任意・スタッフ用の記録。患者への通知には含まれません）" error={errors.reason?.message} {...register('reason')} />
        <Button type="submit" variant="danger">休診を設定する</Button>
      </form>

      {pending && (
        <Dialog title="休診の確認" onClose={() => setPending(null)}>
          <p className="mb-2">この期間の枠は予約できなくなります。すでに予約している方には、休診のお知らせを通知します。</p>
          <p className="mb-4 text-sm text-neutral-600">{pending.doctor_id ? '選択した医師' : '医療機関全体'}の休診として設定します。設定後の取り消しはできません。</p>
          {create.error && <Alert tone="error" className="mb-4">{userMessage(parseApiError(create.error))}</Alert>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setPending(null)}>戻る</Button>
            <Button variant="danger" loading={create.isPending} onClick={confirm}>設定する</Button>
          </div>
        </Dialog>
      )}
    </div>
  );
}
