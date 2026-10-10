import { RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Spinner } from '@/components/Spinner';
import { formatTime } from '@/lib/format';
import { useWaitTimes } from '../api/useWaitTimes';
import { WaitTimeCard } from './WaitTimeCard';
import { WalkInDialog } from './WalkInDialog';

/** 診療科別の待ち時間ボード（未ログインでも閲覧可）。30 秒ごとに自動更新し、失敗時は前回値を残して知らせる。 */
export function WaitTimeBoard() {
  const q = useWaitTimes();
  const [walkIn, setWalkIn] = useState<{ id: number; name: string } | null>(null);

  if (q.isPending) return <Spinner />;
  if (!q.data) {
    return (
      <div className="space-y-3">
        <Alert tone="error">{userMessage(parseApiError(q.error))}</Alert>
        <Button variant="secondary" onClick={() => q.refetch()}>再試行</Button>
      </div>
    );
  }

  const { data } = q;
  return (
    <section aria-label="診療科別の待ち時間" className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-neutral-600">
        <p>最終更新 {formatTime(data.updated_at)}（30秒ごとに自動更新）</p>
        <Button variant="ghost" size="sm" onClick={() => q.refetch()} loading={q.isFetching}>
          <RefreshCw className="h-4 w-4" aria-hidden="true" />
          最新の情報に更新
        </Button>
      </div>
      {q.isError && (
        <Alert tone="warning" urgent>
          最新の情報を取得できませんでした。{formatTime(data.updated_at)}時点の情報を表示しています。
        </Alert>
      )}
      <Alert tone="info">{data.note}</Alert>
      {data.departments.length === 0 ? (
        <p className="py-6 text-center text-neutral-600">現在表示できる診療科がありません</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {data.departments.map((d) => (
            <WaitTimeCard
              key={d.department_id}
              dept={d}
              action={
                d.status === 'available' ? (
                  <Button variant="secondary" aria-label={`${d.name}の当日受付をする`} onClick={() => setWalkIn({ id: d.department_id, name: d.name })}>
                    当日受付をする
                  </Button>
                ) : undefined
              }
            />
          ))}
        </div>
      )}
      {walkIn && <WalkInDialog department={walkIn} onClose={() => setWalkIn(null)} />}
    </section>
  );
}
