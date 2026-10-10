import { useId, useState, type ReactNode } from 'react';
import { cn } from '@/lib/utils';

export interface TabItem {
  id: string;
  label: string;
  content: ReactNode;
}

/** WAI-ARIA の tablist。矢印キーで切り替えられる。非表示のタブの中身は描画しない（不要な API 呼び出しを避ける）。 */
export function Tabs({ items, initial }: { items: TabItem[]; initial?: string }) {
  const base = useId();
  const [active, setActive] = useState(initial ?? items[0]?.id);
  const index = items.findIndex((t) => t.id === active);

  const onKey = (e: React.KeyboardEvent) => {
    const delta = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    if (!delta) return;
    const next = items[(index + delta + items.length) % items.length];
    if (next) {
      setActive(next.id);
      document.getElementById(`${base}-tab-${next.id}`)?.focus();
    }
  };

  return (
    <div>
      <div role="tablist" aria-label="表示の切り替え" onKeyDown={onKey} className="mb-4 flex gap-1 overflow-x-auto border-b border-neutral-200">
        {items.map((t) => (
          <button
            key={t.id}
            id={`${base}-tab-${t.id}`}
            role="tab"
            type="button"
            aria-selected={t.id === active}
            aria-controls={`${base}-panel-${t.id}`}
            tabIndex={t.id === active ? 0 : -1}
            onClick={() => setActive(t.id)}
            className={cn(
              'min-h-11 whitespace-nowrap border-b-2 px-4 text-sm font-medium',
              t.id === active ? 'border-primary-500 text-primary-700' : 'border-transparent text-neutral-600 hover:text-neutral-900',
            )}
          >
            {t.label}
          </button>
        ))}
      </div>
      {items.map((t) =>
        t.id === active ? (
          <div key={t.id} role="tabpanel" id={`${base}-panel-${t.id}`} aria-labelledby={`${base}-tab-${t.id}`}>
            {t.content}
          </div>
        ) : null,
      )}
    </div>
  );
}
