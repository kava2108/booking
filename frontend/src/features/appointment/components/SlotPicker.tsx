import type { Doctor, Slot } from '@/api/types';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Spinner } from '@/components/Spinner';
import { useSlots } from '@/features/catalog/api/useCatalog';
import { formatTime } from '@/lib/format';
import { cn } from '@/lib/utils';

/** 枠が選べない理由（API の unavailable_reason）。 */
const REASON_LABEL: Record<string, string> = {
  full: '満席',
  closed: '休診',
  past_cutoff: '受付終了',
  outside_window: '予約受付期間外',
};

interface Props {
  departmentId: number;
  doctorId?: number | undefined;
  date: string;
  doctors: readonly Doctor[];
  selectedId?: string | undefined;
  onSelect: (slot: Slot) => void;
}

/** 空き枠の一覧。満席・休診・締切後の枠も理由つきで表示し、選択できないようにする（REQ-001-AC-3）。 */
export function SlotPicker({ departmentId, doctorId, date, doctors, selectedId, onSelect }: Props) {
  const q = useSlots({ departmentId, doctorId, date });
  const doctorName = (id: number) => doctors.find((d) => d.id === id)?.name ?? `医師${id}`;

  if (q.isPending) return <Spinner label="空き枠を確認中" />;
  if (q.isError) {
    return (
      <div className="space-y-3">
        <Alert tone="error">{userMessage(parseApiError(q.error))}</Alert>
        <Button variant="secondary" onClick={() => q.refetch()}>再試行</Button>
      </div>
    );
  }
  const slots = q.data;
  if (slots.length === 0) return <p className="py-6 text-center text-neutral-600">この日は予約できる枠がありません</p>;

  const byDoctor = new Map<number, Slot[]>();
  for (const s of slots) byDoctor.set(s.doctor_id, [...(byDoctor.get(s.doctor_id) ?? []), s]);

  return (
    <div className="space-y-5">
      {!slots.some((s) => s.available) && <Alert tone="warning">この日の枠はすべて埋まっているか、受付を終了しています。別の日をお選びください。</Alert>}
      {[...byDoctor.entries()].map(([id, list]) => (
        <section key={id} aria-label={`${doctorName(id)} 医師の空き枠`}>
          <h3 className="mb-2 text-sm font-bold text-neutral-800">{doctorName(id)} 医師</h3>
          <ul className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6">
            {list.map((s) => {
              const reason = s.available ? undefined : (REASON_LABEL[s.unavailable_reason ?? ''] ?? '予約不可');
              return (
                <li key={s.id}>
                  <button
                    type="button"
                    disabled={!s.available}
                    aria-pressed={s.id === selectedId}
                    aria-label={`${formatTime(s.start_at)} ${doctorName(id)}${reason ? ` ${reason}` : ''}`}
                    onClick={() => onSelect(s)}
                    className={cn(
                      'flex min-h-14 w-full flex-col items-center justify-center rounded-lg border px-1 text-sm font-medium transition-colors',
                      s.available
                        ? 'border-primary-300 bg-white text-primary-800 hover:bg-primary-50'
                        : 'cursor-not-allowed border-neutral-200 bg-neutral-100 text-neutral-500',
                      s.id === selectedId && 'border-primary-600 bg-primary-100 ring-2 ring-primary-500',
                    )}
                  >
                    <span className="text-base">{formatTime(s.start_at)}</span>
                    {reason && <span className="text-xs">{reason}</span>}
                  </button>
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </div>
  );
}
