import { Link } from 'react-router-dom';
import { parseApiError, userMessage } from '@/api/errors';
import { Alert } from '@/components/Alert';
import { Button } from '@/components/Button';
import { Dialog } from '@/components/Dialog';
import { Spinner } from '@/components/Spinner';
import { useAuth } from '@/store/auth';
import { useWalkIn } from '../api/useWalkIn';

interface Props {
  department: { id: number; name: string };
  onClose: () => void;
}

/** 当日受付（予約なしで順番に並ぶ）。ログインが必要。 */
export function WalkInDialog({ department, onClose }: Props) {
  const { status } = useAuth();
  const walkIn = useWalkIn();

  return (
    <Dialog title={`${department.name} の当日受付`} onClose={onClose}>
      {status === 'loading' && <Spinner />}
      {status === 'anonymous' && (
        <div className="space-y-4">
          <p>当日受付にはログインが必要です。アカウントをお持ちでない場合は新規登録してください。</p>
          <div className="flex flex-wrap gap-2">
            <Link to="/login" state={{ from: '/' }} className="inline-flex min-h-11 items-center rounded-lg bg-primary-500 px-4 font-medium text-white hover:bg-primary-600">
              ログインする
            </Link>
            <Link to="/register" className="inline-flex min-h-11 items-center rounded-lg border border-primary-300 px-4 font-medium text-primary-700">
              新規登録
            </Link>
          </div>
        </div>
      )}
      {status === 'authenticated' && !walkIn.isSuccess && (
        <div className="space-y-4">
          <p>予約なしで順番に並びます。受付すると受付番号をお知らせします。</p>
          {walkIn.error && <Alert tone="error">{userMessage(parseApiError(walkIn.error))}</Alert>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={onClose}>キャンセル</Button>
            <Button variant="cta" loading={walkIn.isPending} onClick={() => walkIn.mutate(department.id)}>受付する</Button>
          </div>
        </div>
      )}
      {walkIn.isSuccess && (
        <div className="space-y-4">
          <Alert tone="success">受付が完了しました。</Alert>
          <p className="text-3xl font-bold text-primary-700">受付番号 {walkIn.data.ticket_no}</p>
          <p>{walkIn.data.ahead_count > 0 ? `あと${walkIn.data.ahead_count}人でお呼びします。` : 'まもなくお呼びします。'}院内でお待ちください。</p>
          <div className="flex justify-end">
            <Button onClick={onClose}>閉じる</Button>
          </div>
        </div>
      )}
    </Dialog>
  );
}
