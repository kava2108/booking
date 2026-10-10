import type { AppointmentSummary } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Dialog } from '@/components/Dialog';
import { formatDateTime } from '@/lib/format';
import { useCancelAppointment } from '../api/useAppointments';

/** キャンセルの確認。期限を過ぎていた場合はサーバーが拒否し、その理由を表示する。 */
export function CancelDialog({ appointment, onClose, onDone }: { appointment: AppointmentSummary; onClose: () => void; onDone: () => void }) {
  const cancel = useCancelAppointment();
  return (
    <Dialog title="予約のキャンセル" onClose={onClose}>
      <p className="mb-2">次のご予約をキャンセルします。よろしいですか？</p>
      <dl className="mb-4 rounded-lg bg-neutral-50 p-3 text-sm">
        <dt className="sr-only">予約番号</dt><dd className="font-bold">{appointment.appointment_no}</dd>
        <dt className="sr-only">日時</dt><dd>{formatDateTime(appointment.slot_start)}</dd>
        <dt className="sr-only">診療科・担当医</dt><dd>{appointment.department} / {appointment.doctor}</dd>
      </dl>
      {cancel.error && <Alert tone="error" className="mb-4">{userMessage(parseApiError(cancel.error))}</Alert>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>戻る</Button>
        <Button variant="danger" loading={cancel.isPending} onClick={() => cancel.mutate(appointment.id, { onSuccess: onDone })}>
          キャンセルを確定する
        </Button>
      </div>
    </Dialog>
  );
}
