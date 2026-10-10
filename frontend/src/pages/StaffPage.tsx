import { useState } from 'react';
import { Card, PageHeader } from '@/components/Card';
import { SelectInput } from '@/components/FormField';
import { Spinner } from '@/components/Spinner';
import { Tabs } from '@/components/Tabs';
import { useDepartments } from '@/features/catalog/api/useCatalog';
import { ClosureForm } from '@/features/staff/components/ClosureForm';
import { QueueBoard } from '@/features/staff/components/QueueBoard';
import { StaffWalkInForm } from '@/features/staff/components/StaffWalkInForm';
import { TodayAppointments } from '@/features/staff/components/TodayAppointments';
import { usePageTitle } from '@/hooks/usePageTitle';

/** 受付スタッフ画面。患者の氏名・症状を扱うため、API 側で staff / clinic_admin に限定され、閲覧は監査ログに記録される。 */
export function StaffPage() {
  usePageTitle('スタッフ画面');
  const departments = useDepartments();
  const [selected, setSelected] = useState<number | undefined>();
  const deptId = selected ?? departments.data?.[0]?.id;

  return (
    <>
      <PageHeader title="スタッフ画面" description="来院受付・呼出・診察の記録、当日受付の代行、休診の設定を行います。" />
      {departments.isPending && <Spinner />}
      {departments.data && deptId !== undefined && (
        <div className="space-y-4">
          <Card className="max-w-xs">
            <SelectInput label="診療科" value={deptId} onChange={(e) => setSelected(Number(e.target.value))}>
              {departments.data.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
            </SelectInput>
          </Card>
          {/* key で診療科ごとに状態を作り直す（切り替え時に別診療科の入力値や結果を引きずらない） */}
          <Tabs
            key={deptId}
            items={[
              { id: 'appointments', label: '当日の予約', content: <TodayAppointments departmentId={deptId} /> },
              { id: 'queue', label: '待機列', content: <QueueBoard departmentId={deptId} /> },
              { id: 'walkin', label: '当日受付（代行）', content: <StaffWalkInForm departmentId={deptId} /> },
              { id: 'closure', label: '休診設定', content: <ClosureForm departmentId={deptId} /> },
            ]}
          />
        </div>
      )}
    </>
  );
}
