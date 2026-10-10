import { Link } from 'react-router-dom';
import { PageHeader } from '@/components/Card';
import { usePageTitle } from '@/hooks/usePageTitle';

export function NotFoundPage() {
  usePageTitle('ページが見つかりません');
  return (
    <div className="space-y-4">
      <PageHeader title="ページが見つかりません" description="URL をご確認ください。" />
      <Link to="/" className="text-primary-700 underline">トップへ戻る</Link>
    </div>
  );
}
