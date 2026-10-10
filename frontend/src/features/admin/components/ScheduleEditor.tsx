import { zodResolver } from '@hookform/resolvers/zod';
import { useMemo, useState } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { z } from 'zod';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { Dialog } from '@/components/Dialog';
import { SelectInput } from '@/components/FormField';
import { Spinner } from '@/components/Spinner';
import { useDepartments, useDoctors } from '@/features/catalog/api/useCatalog';
import { WEEKDAYS } from '@/lib/format';
import { useAdminSchedules, usePutSchedules } from '../api/useAdmin';

const HHMM = /^([01]\d|2[0-3]):[0-5]\d$/;
const minutes = (hhmm: string): number => Number(hhmm.slice(0, 2)) * 60 + Number(hhmm.slice(3, 5));

const rowSchema = z.object({
  weekday: z.string(),
  start: z.string().regex(HHMM, '開始時刻を入力してください'),
  end: z.string().regex(HHMM, '終了時刻を入力してください'),
  slot: z.string().regex(/^[1-9]\d*$/, '1以上の整数で入力してください'),
  capacity: z.string().regex(/^[1-9]\d*$/, '1以上の整数で入力してください'),
});

// バックエンド(admin_service.go validateSchedules)と同じ規則を、送信前に画面で検証する
const schema = z.object({ schedules: z.array(rowSchema) }).superRefine((v, ctx) => {
  const ok = v.schedules.map((r) => HHMM.test(r.start) && HHMM.test(r.end));
  v.schedules.forEach((r, i) => {
    if (!ok[i]) return;
    const s = minutes(r.start), e = minutes(r.end);
    if (e <= s) ctx.addIssue({ code: 'custom', path: ['schedules', i, 'end'], message: '終了は開始より後にしてください' });
    else if (/^[1-9]\d*$/.test(r.slot) && Number(r.slot) > e - s) ctx.addIssue({ code: 'custom', path: ['schedules', i, 'slot'], message: '枠の長さが診療時間を超えています' });
  });
  // 同じ曜日の時間帯が重ならない（隣接は可）
  for (let i = 0; i < v.schedules.length; i++) {
    for (let j = i + 1; j < v.schedules.length; j++) {
      const a = v.schedules[i]!, b = v.schedules[j]!;
      if (a.weekday !== b.weekday || !ok[i] || !ok[j]) continue;
      if (minutes(a.start) < minutes(b.end) && minutes(b.start) < minutes(a.end) && minutes(a.end) > minutes(a.start) && minutes(b.end) > minutes(b.start)) {
        ctx.addIssue({ code: 'custom', path: ['schedules', j, 'start'], message: '同じ曜日の他の時間帯と重なっています' });
      }
    }
  }
});
type Values = z.infer<typeof schema>;

const input = 'min-h-11 w-full rounded-lg border border-neutral-300 bg-white px-2 text-base aria-[invalid=true]:border-red-500';

/** 医師ごとの診療スケジュール（曜日・診療時間・枠の長さ・定員）。保存すると枠が自動生成される。 */
export function ScheduleEditor() {
  const departments = useDepartments();
  const [deptId, setDeptId] = useState<number | undefined>();
  const [doctorId, setDoctorId] = useState<number | undefined>();
  const doctors = useDoctors(deptId);
  const schedules = useAdminSchedules(doctorId);

  return (
    <div className="space-y-6">
      <Card className="grid gap-4 sm:grid-cols-2">
        <SelectInput label="診療科" value={deptId ?? ''} onChange={(e) => { setDeptId(e.target.value ? Number(e.target.value) : undefined); setDoctorId(undefined); }}>
          <option value="">選択してください</option>
          {departments.data?.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </SelectInput>
        <SelectInput label="医師" disabled={deptId === undefined} value={doctorId ?? ''} onChange={(e) => setDoctorId(e.target.value ? Number(e.target.value) : undefined)}>
          <option value="">選択してください</option>
          {doctors.data?.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </SelectInput>
      </Card>

      {doctorId === undefined && <p className="py-6 text-center text-neutral-600">診療科と医師を選ぶと、診療スケジュールを編集できます</p>}
      {doctorId !== undefined && schedules.isPending && <Spinner />}
      {doctorId !== undefined && schedules.isError && (
        <div className="space-y-3">
          <Alert tone="error">{userMessage(parseApiError(schedules.error))}</Alert>
          <Button variant="secondary" onClick={() => schedules.refetch()}>再試行</Button>
        </div>
      )}
      {doctorId !== undefined && schedules.data && (
        <Editor key={doctorId} doctorId={doctorId} initial={schedules.data.schedules.map((s) => ({ weekday: String(s.weekday), start: s.start_time, end: s.end_time, slot: String(s.slot_minutes), capacity: String(s.capacity) }))} />
      )}
    </div>
  );
}

function Editor({ doctorId, initial }: { doctorId: number; initial: Values['schedules'] }) {
  const save = usePutSchedules();
  const [confirmEmpty, setConfirmEmpty] = useState<Values | null>(null);
  const defaults = useMemo(() => ({ schedules: initial }), [initial]);
  const { register, control, handleSubmit, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: defaults });
  const { fields, append, remove } = useFieldArray({ control, name: 'schedules' });

  const submit = (v: Values) =>
    save.mutate({
      doctor_id: doctorId,
      schedules: v.schedules.map((r) => ({ weekday: Number(r.weekday), start_time: r.start, end_time: r.end, slot_minutes: Number(r.slot), capacity: Number(r.capacity) })),
    });
  // すべて削除して保存すると診療がなくなる。誤操作を防ぐため確認する
  const onValid = (v: Values) => (v.schedules.length === 0 ? setConfirmEmpty(v) : submit(v));

  return (
    <form onSubmit={handleSubmit(onValid)} noValidate className="space-y-4">
      {save.error && <Alert tone="error">{userMessage(parseApiError(save.error))}</Alert>}
      {save.isSuccess && <Alert tone="success">保存しました。診療時間にあわせて予約枠を生成しました。</Alert>}
      <Card className="space-y-4">
        {fields.length === 0 && <p className="text-neutral-600">診療時間が設定されていません。</p>}
        {fields.map((f, i) => {
          const n = i + 1;
          const e = errors.schedules?.[i];
          return (
            <div key={f.id} className="grid grid-cols-2 gap-3 border-b border-neutral-100 pb-4 sm:grid-cols-6">
              <label className="text-xs text-neutral-600">曜日
                <select aria-label={`曜日（${n}行目）`} className={input} {...register(`schedules.${i}.weekday`)}>
                  {WEEKDAYS.map((w, d) => <option key={w} value={d}>{w}曜日</option>)}
                </select>
              </label>
              <label className="text-xs text-neutral-600">開始
                <input type="time" aria-label={`開始（${n}行目）`} aria-invalid={!!e?.start} className={input} {...register(`schedules.${i}.start`)} />
              </label>
              <label className="text-xs text-neutral-600">終了
                <input type="time" aria-label={`終了（${n}行目）`} aria-invalid={!!e?.end} className={input} {...register(`schedules.${i}.end`)} />
              </label>
              <label className="text-xs text-neutral-600">枠の長さ（分）
                <input type="number" inputMode="numeric" aria-label={`枠の長さ・分（${n}行目）`} aria-invalid={!!e?.slot} className={input} {...register(`schedules.${i}.slot`)} />
              </label>
              <label className="text-xs text-neutral-600">定員
                <input type="number" inputMode="numeric" aria-label={`定員（${n}行目）`} aria-invalid={!!e?.capacity} className={input} {...register(`schedules.${i}.capacity`)} />
              </label>
              <div className="flex items-end">
                <Button variant="ghost" aria-label={`${n}行目を削除`} onClick={() => remove(i)}>削除</Button>
              </div>
              {[e?.start, e?.end, e?.slot, e?.capacity].map((x, k) => x?.message && <p key={k} role="alert" className="col-span-full text-sm text-red-700">{x.message}</p>)}
            </div>
          );
        })}
        <Button variant="secondary" onClick={() => append({ weekday: '1', start: '09:00', end: '12:00', slot: '30', capacity: '1' })}>時間帯を追加</Button>
      </Card>
      <Button type="submit" loading={save.isPending}>保存する</Button>

      {confirmEmpty && (
        <Dialog title="診療なしにしますか？" onClose={() => setConfirmEmpty(null)}>
          <p className="mb-4">すべての時間帯を削除すると、この医師は診療なしとなり、新しい予約枠は作られなくなります（予約済みの枠は残ります）。</p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setConfirmEmpty(null)}>戻る</Button>
            <Button variant="danger" onClick={() => { submit(confirmEmpty); setConfirmEmpty(null); }}>保存する</Button>
          </div>
        </Dialog>
      )}
    </form>
  );
}
