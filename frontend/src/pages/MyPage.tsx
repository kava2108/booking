import { Link } from 'react-router-dom';
import { PageHeader } from '@/components/Card';
import { AppointmentList } from '@/features/appointment/components/AppointmentList';
import { usePageTitle } from '@/hooks/usePageTitle';

export function MyPage() {
  usePageTitle('マイページ');
  return (
    <>
      <PageHeader
        title="マイページ"
        description="ご予約の確認・日時の変更・キャンセルができます。"
        actions={<Link to="/reserve" className="inline-flex min-h-11 items-center rounded-lg bg-accent-500 px-5 font-semibold text-neutral-900 hover:bg-accent-600">新しく予約する</Link>}
      />
      <AppointmentList />
    </>
  );
}
