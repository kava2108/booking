import { useState } from 'react';
import { Link } from 'react-router-dom';
import type { AppointmentSummary } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Spinner } from '@/components/Spinner';
import { ymdJst } from '@/lib/format';
import { useMyAppointments } from '../api/useAppointments';
import { AppointmentCard } from './AppointmentCard';
import { CancelDialog } from './CancelDialog';
import { RescheduleDialog } from './RescheduleDialog';

/** マイページの予約一覧。今後の予約と、これまでの予約（過去・キャンセル済み）を分けて表示する。 */
export function AppointmentList() {
  const q = useMyAppointments();
  const [canceling, setCanceling] = useState<AppointmentSummary | null>(null);
  const [rescheduling, setRescheduling] = useState<AppointmentSummary | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  if (q.isPending) return <Spinner />;
  if (q.isError) {
    return (
      <div className="space-y-3">
        <Alert tone="error">{userMessage(parseApiError(q.error))}</Alert>
        <Button variant="secondary" onClick={() => q.refetch()}>再試行</Button>
      </div>
    );
  }
  if (q.data.length === 0) {
    return (
      <div className="space-y-3 py-8 text-center">
        <p className="text-neutral-700">ご予約はありません</p>
        <Link to="/reserve" className="inline-flex min-h-11 items-center rounded-lg bg-accent-500 px-5 font-semibold text-neutral-900 hover:bg-accent-600">予約する</Link>
      </div>
    );
  }

  // 診療日（JST）が今日以降の有効な予約を「今後」とする（本日の予約は時刻を過ぎても今日の間は表示）
  const today = ymdJst();
  const isUpcoming = (a: AppointmentSummary) => a.status === 'booked' && ymdJst(new Date(a.slot_start)) >= today;
  const upcoming = q.data.filter(isUpcoming).sort((a, b) => a.slot_start.localeCompare(b.slot_start));
  const history = q.data.filter((a) => !isUpcoming(a)).sort((a, b) => b.slot_start.localeCompare(a.slot_start));

  const group = (title: string, items: AppointmentSummary[], up: boolean) => (
    <section aria-label={title} className="space-y-3">
      <h2 className="text-lg font-bold text-neutral-900">{title}</h2>
      {items.length === 0 ? <p className="text-sm text-neutral-600">該当するご予約はありません</p> : items.map((a) => (
        <AppointmentCard key={a.id} appointment={a} upcoming={up} onCancel={() => setCanceling(a)} onReschedule={() => setRescheduling(a)} />
      ))}
    </section>
  );

  return (
    <div className="space-y-8">
      {notice && <Alert tone="success">{notice}</Alert>}
      {group('今後のご予約', upcoming, true)}
      {group('これまでのご予約', history, false)}
      {canceling && <CancelDialog appointment={canceling} onClose={() => setCanceling(null)} onDone={() => { setCanceling(null); setNotice('予約をキャンセルしました。'); }} />}
      {rescheduling && <RescheduleDialog appointment={rescheduling} onClose={() => setRescheduling(null)} />}
    </div>
  );
}
