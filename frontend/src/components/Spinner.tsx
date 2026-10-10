import { Loader2 } from 'lucide-react';

export function Spinner({ label = '読み込み中' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-8 text-neutral-600">
      <Loader2 className="h-5 w-5 animate-spin" aria-hidden="true" />
      <span>{label}…</span>
    </div>
  );
}
