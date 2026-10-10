import type { ReactNode } from 'react';
import type { DepartmentWaitTime } from '@/api/types';
import { Card } from '@/components/Card';
import { formatWaitMinutes } from '@/lib/format';

const REASONS: Record<string, string> = {
  before_hours: '診療開始前です。診療が始まると待ち時間が表示されます。',
  closed: '現在、休診または診療時間外です。',
  no_data: '現在は待ち時間を算出できません（診察実績が不足しています）。',
};

const BASIS: Record<string, string> = {
  actual: '直近の診察実績をもとに計算しています。',
  standard: '標準的な診察時間をもとにした目安です。',
};

/**
 * 診療科 1 つぶんの待ち時間。算出不可のときは理由だけを表示し、数値は一切出さない（推測しない）。
 * 個人を特定できる情報は API から返らず、画面にも出さない。
 */
export function WaitTimeCard({ dept, action }: { dept: DepartmentWaitTime; action?: ReactNode }) {
  const headingId = `dept-${dept.department_id}`;
  const available = dept.status === 'available';
  return (
    <Card aria-labelledby={headingId} role="group" className="flex flex-col gap-3">
      <h3 id={headingId} className="text-lg font-bold text-neutral-900">{dept.name}</h3>
      {available ? (
        <>
          {(dept.waiting_count ?? 0) === 0 ? (
            <p className="text-lg font-semibold text-primary-700">現在お待ちの方はいません</p>
          ) : (
            <dl className="flex items-end gap-6">
              <div>
                <dt className="text-xs text-neutral-600">待ち時間の目安</dt>
                <dd className="text-3xl font-bold text-primary-700">{formatWaitMinutes(dept.wait_minutes ?? 0)}</dd>
              </div>
              <div>
                <dt className="text-xs text-neutral-600">お待ちの方</dt>
                <dd className="text-xl font-semibold text-neutral-900">{dept.waiting_count}人</dd>
              </div>
            </dl>
          )}
          {dept.basis && <p className="text-xs text-neutral-600">{BASIS[dept.basis]}</p>}
        </>
      ) : (
        <p className="text-neutral-700">{REASONS[dept.reason ?? ''] ?? '現在はご利用いただけません。'}</p>
      )}
      {action}
    </Card>
  );
}
