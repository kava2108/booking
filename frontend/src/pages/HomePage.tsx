import { Link } from 'react-router-dom';
import { PageHeader } from '@/components/Card';
import { WaitTimeBoard } from '@/features/waittime/components/WaitTimeBoard';
import { usePageTitle } from '@/hooks/usePageTitle';

export function HomePage() {
  usePageTitle('現在の待ち時間');
  return (
    <>
      <PageHeader
        title="現在の待ち時間"
        description="診療科ごとのお待ちの人数と、待ち時間の目安です。"
        actions={<Link to="/reserve" className="inline-flex min-h-11 items-center rounded-lg bg-accent-500 px-5 font-semibold text-neutral-900 hover:bg-accent-600">診察を予約する</Link>}
      />
      <WaitTimeBoard />
    </>
  );
}
