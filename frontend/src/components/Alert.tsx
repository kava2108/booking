import { AlertTriangle, CheckCircle2, Info, XCircle } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type Tone = 'error' | 'success' | 'info' | 'warning';

const tones: Record<Tone, { box: string; icon: typeof Info }> = {
  error: { box: 'border-red-300 bg-red-50 text-red-900', icon: XCircle },
  success: { box: 'border-green-300 bg-green-50 text-green-900', icon: CheckCircle2 },
  info: { box: 'border-blue-300 bg-blue-50 text-blue-900', icon: Info },
  warning: { box: 'border-yellow-400 bg-yellow-50 text-yellow-900', icon: AlertTriangle },
};

/** エラー（または urgent）は role="alert"（即時に読み上げ）、それ以外は role="status"。 */
export function Alert({ tone = 'info', children, className, urgent }: { tone?: Tone; children: ReactNode; className?: string; urgent?: boolean }) {
  const { box, icon: Icon } = tones[tone];
  return (
    <div role={tone === 'error' || urgent ? 'alert' : 'status'} className={cn('flex gap-2 rounded-lg border p-3 text-sm', box, className)}>
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div>{children}</div>
    </div>
  );
}
