import { PageHeader } from '@/components/Card';
import { BookingFlow } from '@/features/appointment/components/BookingFlow';
import { usePageTitle } from '@/hooks/usePageTitle';

export function ReservePage() {
  usePageTitle('予約する');
  return (
    <>
      <PageHeader title="予約する" description="診療科と日付を選んで、空いている時間を予約できます。予約の確定にはログインが必要です。" />
      <BookingFlow />
    </>
  );
}
