import { useState } from 'react';
import type { AppointmentSummary, Slot } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Dialog } from '@/components/Dialog';
import { TextInput } from '@/components/FormField';
import { useDepartments, useDoctors } from '@/features/catalog/api/useCatalog';
import { formatDateTime, ymdJst } from '@/lib/format';
import { useRescheduleAppointment } from '../api/useAppointments';
import { SlotPicker } from './SlotPicker';

/** 日時の変更。同じ診療科の別の枠を選ぶ。新しい枠が確保できなければ元の予約はそのまま残る。 */
export function RescheduleDialog({ appointment, onClose }: { appointment: AppointmentSummary; onClose: () => void }) {
  const today = ymdJst();
  const [date, setDate] = useState(today);
  const [chosen, setChosen] = useState<Slot | null>(null);
  const reschedule = useRescheduleAppointment();
  // 予約には診療科名しか含まれないため、名前（一意）から ID を引く
  const departments = useDepartments();
  const deptId = departments.data?.find((d) => d.name === appointment.department)?.id;
  const doctors = useDoctors(deptId);

  if (reschedule.isSuccess) {
    return (
      <Dialog title="日時の変更" onClose={onClose}>
        <Alert tone="success"><strong>日時を変更しました</strong></Alert>
        <p className="mt-3">{formatDateTime(reschedule.data.slot_start)}（{reschedule.data.department} / {reschedule.data.doctor}）</p>
        <div className="mt-4 flex justify-end"><Button onClick={onClose}>閉じる</Button></div>
      </Dialog>
    );
  }

  return (
    <Dialog title="日時の変更" onClose={onClose}>
      <p className="mb-3 text-sm text-neutral-600">現在のご予約: {formatDateTime(appointment.slot_start)}（{appointment.department} / {appointment.doctor}）</p>
      <div className="mb-4">
        <TextInput label="変更したい日付" type="date" min={today} value={date} onChange={(e) => e.target.value && e.target.value >= today && (setDate(e.target.value), setChosen(null))} />
      </div>
      {deptId !== undefined && <SlotPicker departmentId={deptId} date={date} doctors={doctors.data ?? []} selectedId={chosen?.id} onSelect={setChosen} />}
      {reschedule.error && <Alert tone="error" className="mt-4">{userMessage(parseApiError(reschedule.error))}</Alert>}
      <div className="mt-4 flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>戻る</Button>
        <Button variant="cta" disabled={!chosen} loading={reschedule.isPending} onClick={() => chosen && reschedule.mutate({ id: appointment.id, slotId: chosen.id })}>
          この日時に変更する
        </Button>
      </div>
    </Dialog>
  );
}
