import type { StaffQueueEntry } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Badge } from '@/components/Badge';
import { Button } from '@/components/Button';
import { Spinner } from '@/components/Spinner';
import { formatTime } from '@/lib/format';
import { useQueueAction, useStaffQueue, type QueueAction } from '../api/useStaff';

const STATUS: Record<string, { label: string; tone: 'neutral' | 'primary' | 'warning' | 'success' }> = {
  waiting: { label: '待機中', tone: 'neutral' },
  called: { label: '呼出済み', tone: 'warning' },
  in_consultation: { label: '診察中', tone: 'primary' },
};

// 状態ごとに「次にやる 1 つの操作」だけを出す（1 タップで記録できる、押し間違いを減らす）
const NEXT: Record<string, { action: QueueAction; label: string }> = {
  waiting: { action: 'call', label: '呼出' },
  called: { action: 'start', label: '診察開始' },
  in_consultation: { action: 'complete', label: '診察完了' },
};

/** 待機列。氏名・症状を含むのでスタッフ専用画面でのみ使う。15 秒ごとに自動更新する。 */
export function QueueBoard({ departmentId }: { departmentId: number }) {
  const q = useStaffQueue(departmentId);
  const op = useQueueAction();

  if (q.isPending) return <Spinner />;
  if (q.isError && !q.data) return <Alert tone="error">{userMessage(parseApiError(q.error))}</Alert>;
  const rows: StaffQueueEntry[] = q.data ?? [];

  return (
    <div className="space-y-3">
      {op.error && <Alert tone="error">{userMessage(parseApiError(op.error))}</Alert>}
      {rows.length === 0 ? (
        <p className="py-8 text-center text-neutral-600">現在、待機中の方はいません</p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-neutral-200 bg-white">
          <table className="w-full text-left text-sm">
            <caption className="sr-only">待機列</caption>
            <thead className="bg-neutral-50 text-xs text-neutral-600">
              <tr>
                <th scope="col" className="px-3 py-2">番号</th>
                <th scope="col" className="px-3 py-2">お名前</th>
                <th scope="col" className="px-3 py-2">種別</th>
                <th scope="col" className="px-3 py-2">状態</th>
                <th scope="col" className="px-3 py-2">症状など</th>
                <th scope="col" className="px-3 py-2">受付</th>
                <th scope="col" className="px-3 py-2"><span className="sr-only">操作</span></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => {
                const st = STATUS[r.status] ?? { label: r.status, tone: 'neutral' as const };
                const next = NEXT[r.status];
                const busy = op.isPending && op.variables?.id === r.id;
                return (
                  <tr key={r.id} className="border-t border-neutral-100">
                    <td className="px-3 py-2 font-bold">{r.ticket_no}</td>
                    <td className="px-3 py-2">{r.patient_name}</td>
                    <td className="px-3 py-2">{r.source === 'reservation' ? '予約' : '当日受付'}</td>
                    <td className="px-3 py-2"><Badge tone={st.tone}>{st.label}</Badge></td>
                    <td className="max-w-48 px-3 py-2 text-neutral-700">{r.symptom_note}</td>
                    <td className="px-3 py-2 text-neutral-600">{r.checked_in_at ? formatTime(r.checked_in_at) : ''}</td>
                    <td className="px-3 py-2">
                      {next && (
                        <Button size="sm" variant={next.action === 'complete' ? 'secondary' : 'primary'} disabled={busy} loading={busy} aria-label={`${r.patient_name}さんの${next.label}`} onClick={() => op.mutate({ id: r.id, action: next.action })}>
                          {next.label}
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
