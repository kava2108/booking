import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Badge } from '@/components/Badge';
import { Button } from '@/components/Button';
import { Spinner } from '@/components/Spinner';
import { formatTime, ymdJst } from '@/lib/format';
import { useCheckIn, useStaffAppointments } from '../api/useStaff';

/** 当日の予約一覧。予約患者の来院受付（待機列への追加）を行う。 */
export function TodayAppointments({ departmentId }: { departmentId: number }) {
  const q = useStaffAppointments(departmentId, ymdJst());
  const checkIn = useCheckIn();

  if (q.isPending) return <Spinner />;
  if (q.isError && !q.data) return <Alert tone="error">{userMessage(parseApiError(q.error))}</Alert>;
  const rows = q.data ?? [];

  const error = checkIn.error ? parseApiError(checkIn.error) : null;
  const message = error
    ? error.code === 'validation_failed'
      ? checkInReason(error.fields.id)
      : userMessage(error)
    : null;

  return (
    <div className="space-y-3">
      {message && <Alert tone="error">{message}</Alert>}
      {rows.length === 0 ? (
        <p className="py-8 text-center text-neutral-600">本日のご予約はありません</p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-neutral-200 bg-white">
          <table className="w-full text-left text-sm">
            <caption className="sr-only">本日の予約</caption>
            <thead className="bg-neutral-50 text-xs text-neutral-600">
              <tr>
                <th scope="col" className="px-3 py-2">時刻</th>
                <th scope="col" className="px-3 py-2">お名前</th>
                <th scope="col" className="px-3 py-2">担当医</th>
                <th scope="col" className="px-3 py-2">来院区分</th>
                <th scope="col" className="px-3 py-2">状態</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((a) => {
                const busy = checkIn.isPending && checkIn.variables === a.id;
                return (
                  <tr key={a.id} className="border-t border-neutral-100">
                    <td className="px-3 py-2 font-bold">{formatTime(a.slot_start)}</td>
                    <td className="px-3 py-2">{a.patient_name}</td>
                    <td className="px-3 py-2">{a.doctor}</td>
                    <td className="px-3 py-2">{a.visit_type === 'first' ? '初診' : '再診'}</td>
                    <td className="px-3 py-2">
                      {a.status === 'completed' ? (
                        <Badge tone="success">診察済み</Badge>
                      ) : a.checked_in ? (
                        <Badge tone="primary">受付済み</Badge>
                      ) : (
                        <Button size="sm" variant="cta" loading={busy} disabled={busy} aria-label={`${a.patient_name}さんの来院受付`} onClick={() => checkIn.mutate(a.id)}>
                          来院受付
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function checkInReason(msg: string | undefined): string {
  if (msg?.includes('not for today')) return 'この予約は本日のものではないため、来院受付できません。';
  if (msg?.includes('not active')) return 'この予約は取り消し済み、または対応済みのため、来院受付できません。';
  return '来院受付できませんでした。予約の内容を確認してください。';
}
