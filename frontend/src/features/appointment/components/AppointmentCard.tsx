import type { AppointmentSummary } from '@/api/types';
import { Badge } from '@/components/Badge';
import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { PositionPanel } from '@/features/waittime/components/PositionPanel';
import { formatDateTime, ymdJst } from '@/lib/format';

const STATUS: Record<string, { label: string; tone: 'primary' | 'success' | 'neutral' | 'warning' }> = {
  booked: { label: '予約済み', tone: 'primary' },
  completed: { label: '診察済み', tone: 'success' },
  canceled: { label: 'キャンセル済み', tone: 'neutral' },
  no_show: { label: '来院なし', tone: 'warning' },
};

interface Props {
  appointment: AppointmentSummary;
  /** これから受診する予約か（過去・取消済みは操作を出さない） */
  upcoming: boolean;
  onCancel: () => void;
  onReschedule: () => void;
}

export function AppointmentCard({ appointment: a, upcoming, onCancel, onReschedule }: Props) {
  const st = STATUS[a.status] ?? { label: a.status, tone: 'neutral' as const };
  const isToday = a.status === 'booked' && ymdJst(new Date(a.slot_start)) === ymdJst();
  return (
    <Card className="space-y-2" role="article" aria-label={`${formatDateTime(a.slot_start)} ${a.department}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-lg font-bold text-neutral-900">{formatDateTime(a.slot_start)}</p>
        <Badge tone={st.tone}>{st.label}</Badge>
      </div>
      <p className="text-sm text-neutral-700">{a.department} / {a.doctor} / {a.visit_type === 'first' ? '初診' : '再診'}</p>
      <p className="text-xs text-neutral-600">予約番号 <span className="font-mono text-sm text-neutral-900">{a.appointment_no}</span></p>
      {a.symptom_note && <p className="text-sm text-neutral-700">症状など: {a.symptom_note}</p>}
      {isToday && <PositionPanel appointmentId={a.id} />}
      {upcoming && a.status === 'booked' &&
        (a.cancelable ? (
          <div className="flex flex-wrap gap-2 pt-1">
            <Button variant="secondary" onClick={onReschedule}>日時を変更する</Button>
            <Button variant="ghost" onClick={onCancel}>キャンセルする</Button>
          </div>
        ) : (
          <p className="text-sm text-neutral-700">キャンセル・変更の期限を過ぎています。変更・キャンセルをご希望の場合は、医療機関へお電話ください。</p>
        ))}
    </Card>
  );
}
