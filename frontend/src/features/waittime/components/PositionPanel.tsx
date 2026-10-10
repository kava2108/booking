import { Alert } from '@/components/Alert';
import { formatTime } from '@/lib/format';
import { usePosition } from '../api/usePosition';

/** 予約した本人の順番と呼出見込み（REQ-004-AC-4）。見込み時刻を算出できないときは推測せず人数だけ伝える。 */
export function PositionPanel({ appointmentId }: { appointmentId: string }) {
  const q = usePosition(appointmentId);

  if (q.isError) return <p className="text-sm text-neutral-600">順番を取得できませんでした。しばらくしてからもう一度ご確認ください。</p>;
  if (!q.data) return null;
  const p = q.data;

  let body: string;
  switch (p.status) {
    case 'not_checked_in':
      body = '来院されましたら受付でお声がけください。来院受付後に順番を表示します。';
      break;
    case 'waiting':
      body = p.ahead_count > 0 ? `あと${p.ahead_count}人でお呼びします。` : '次にお呼びします。';
      break;
    case 'called':
      body = 'お呼びしています。診察室へお進みください。';
      break;
    case 'in_consultation':
      body = '診察中です。';
      break;
    case 'done':
      body = '診察は終了しました。';
      break;
    default:
      body = 'この受付は取り消されました。';
  }
  return (
    <Alert tone="info">
      <p aria-live="polite">{body}</p>
      {p.status === 'waiting' && (
        <p className="mt-1 text-xs">
          {p.estimated_call_at ? `${formatTime(p.estimated_call_at)}ごろの見込みです（目安）。` : '見込み時刻は算出できません。'}
        </p>
      )}
    </Alert>
  );
}
