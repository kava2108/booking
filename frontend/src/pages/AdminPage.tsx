import { PageHeader } from '@/components/Card';
import { Tabs } from '@/components/Tabs';
import { ScheduleEditor } from '@/features/admin/components/ScheduleEditor';
import { SettingsForm } from '@/features/admin/components/SettingsForm';
import { usePageTitle } from '@/hooks/usePageTitle';

export function AdminPage() {
  usePageTitle('管理画面');
  return (
    <>
      <PageHeader title="管理画面" description="診療スケジュールと、予約のルール（受付期間・キャンセル期限・標準診察時間など）を設定します。" />
      <Tabs
        items={[
          { id: 'schedule', label: '診療スケジュール', content: <ScheduleEditor /> },
          { id: 'settings', label: '基本設定', content: <SettingsForm /> },
        ]}
      />
    </>
  );
}
