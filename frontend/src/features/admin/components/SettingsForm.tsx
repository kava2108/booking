import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import type { DepartmentSettings, SettingsRequest } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { TextInput } from '@/components/FormField';
import { Spinner } from '@/components/Spinner';
import { useAdminSettings, useUpdateSettings } from '../api/useAdmin';

// バックエンドの検証範囲（admin_service.go）と揃える
const MAX_CANCEL_MINUTES = 60 * 24 * 31;

/** 数値入力（文字列で受けて検証）。blank を許すときは空欄＝未設定。 */
function int(opts: { min: number; max?: number; blank?: boolean; minMsg: string; maxMsg?: string }) {
  return z.string().superRefine((v, ctx) => {
    const s = v.trim();
    if (s === '') {
      if (!opts.blank) ctx.addIssue({ code: 'custom', message: '入力してください' });
      return;
    }
    const n = Number(s);
    if (!Number.isInteger(n)) return ctx.addIssue({ code: 'custom', message: '整数で入力してください' });
    if (n < opts.min) return ctx.addIssue({ code: 'custom', message: opts.minMsg });
    if (opts.max !== undefined && n > opts.max) ctx.addIssue({ code: 'custom', message: opts.maxMsg ?? opts.minMsg });
  });
}

const schema = z.object({
  cancel: int({ min: 0, max: MAX_CANCEL_MINUTES, minMsg: '0以上で入力してください', maxMsg: `${MAX_CANCEL_MINUTES}以下で入力してください` }),
  window: int({ min: 1, max: 365, minMsg: '1以上365以下で入力してください' }),
  departments: z.array(
    z.object({
      id: z.number(),
      name: z.string(),
      std: int({ min: 1, blank: true, minMsg: '1以上で入力してください' }),
      cutoff: z.string().refine((v) => v === '' || /^([01]\d|2[0-3]):[0-5]\d$/.test(v), '時刻（HH:MM）で入力してください'),
      cap: int({ min: 1, blank: true, minMsg: '1以上で入力してください' }),
    }),
  ),
});
type Values = z.infer<typeof schema>;

const str = (n: number | null | undefined): string => (n === null || n === undefined ? '' : String(n));

function toValues(s: { cancel_deadline_minutes: number; booking_window_days: number; departments?: DepartmentSettings[] | undefined }): Values {
  return {
    cancel: String(s.cancel_deadline_minutes),
    window: String(s.booking_window_days),
    departments: (s.departments ?? []).map((d) => ({ id: d.id, name: d.name, std: str(d.standard_consult_minutes), cutoff: d.walkin_cutoff ?? '', cap: str(d.walkin_capacity) })),
  };
}

/** 空欄の項目は送らない（サーバー側で「未設定」に戻る）。 */
function toRequest(v: Values): SettingsRequest {
  return {
    cancel_deadline_minutes: Number(v.cancel),
    booking_window_days: Number(v.window),
    departments: v.departments.map((d) => ({
      id: d.id,
      ...(d.std.trim() !== '' ? { standard_consult_minutes: Number(d.std) } : {}),
      ...(d.cutoff !== '' ? { walkin_cutoff: d.cutoff } : {}),
      ...(d.cap.trim() !== '' ? { walkin_capacity: Number(d.cap) } : {}),
    })),
  };
}

/** 受付期間・キャンセル期限・診療科ごとの標準診察時間／当日受付の締切・定員。 */
export function SettingsForm() {
  const q = useAdminSettings();
  const update = useUpdateSettings();

  if (q.isPending) return <Spinner />;
  if (q.isError || !q.data) {
    return (
      <div className="space-y-3">
        <Alert tone="error">{userMessage(parseApiError(q.error))}</Alert>
        <Button variant="secondary" onClick={() => q.refetch()}>再試行</Button>
      </div>
    );
  }
  return <Inner initial={toValues(q.data)} update={update} />;
}

function Inner({ initial, update }: { initial: Values; update: ReturnType<typeof useUpdateSettings> }) {
  const { register, handleSubmit, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: initial });
  const err = update.error ? parseApiError(update.error) : null;

  return (
    <form onSubmit={handleSubmit((v) => update.mutate(toRequest(v)))} noValidate className="max-w-2xl space-y-6">
      {err && <Alert tone="error">{userMessage(err)}</Alert>}
      {update.isSuccess && <Alert tone="success">保存しました。</Alert>}

      <Card className="space-y-4">
        <h2 className="text-lg font-bold">予約のルール</h2>
        <TextInput label="キャンセル期限（診察の何分前まで）" type="number" inputMode="numeric" hint="この時間を過ぎると、患者はキャンセル・変更できません。" error={errors.cancel?.message} {...register('cancel')} />
        <TextInput label="予約受付期間（何日先まで）" type="number" inputMode="numeric" hint="今日からこの日数先までの枠を予約できます。" error={errors.window?.message} {...register('window')} />
      </Card>

      {initial.departments.map((d, i) => (
        <Card key={d.id} className="space-y-4">
          <h2 className="text-lg font-bold">{d.name}</h2>
          <TextInput label={`${d.name}の標準診察時間（分）`} type="number" inputMode="numeric" hint="診察の実績が少ないときの待ち時間の計算に使います。空欄＝未設定（実績が無いと待ち時間は算出不可）。" error={errors.departments?.[i]?.std?.message} {...register(`departments.${i}.std`)} />
          <TextInput label={`${d.name}の当日受付の締切時刻`} type="time" hint="この時刻以降は当日受付を終了します。空欄＝締切なし。" error={errors.departments?.[i]?.cutoff?.message} {...register(`departments.${i}.cutoff`)} />
          <TextInput label={`${d.name}の当日受付の定員（人）`} type="number" inputMode="numeric" hint="1日あたりの当日受付の上限。空欄＝上限なし。" error={errors.departments?.[i]?.cap?.message} {...register(`departments.${i}.cap`)} />
        </Card>
      ))}

      <Button type="submit" loading={update.isPending}>保存する</Button>
    </form>
  );
}
