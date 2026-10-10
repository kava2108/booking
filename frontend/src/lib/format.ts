// 日時の表示は常に JST（診療所の時刻）。ブラウザのタイムゾーンに依存しない。
const TZ = 'Asia/Tokyo';

const dateFmt = new Intl.DateTimeFormat('ja-JP', { timeZone: TZ, month: 'long', day: 'numeric', weekday: 'short' });
const dateLongFmt = new Intl.DateTimeFormat('ja-JP', { timeZone: TZ, year: 'numeric', month: 'long', day: 'numeric', weekday: 'short' });
const timeFmt = new Intl.DateTimeFormat('ja-JP', { timeZone: TZ, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
const ymdFmt = new Intl.DateTimeFormat('en-CA', { timeZone: TZ, year: 'numeric', month: '2-digit', day: '2-digit' });

export const formatDate = (iso: string | Date): string => dateFmt.format(new Date(iso));
export const formatDateLong = (iso: string | Date): string => dateLongFmt.format(new Date(iso));
export const formatTime = (iso: string | Date): string => timeFmt.format(new Date(iso));
export const formatDateTime = (iso: string | Date): string => `${formatDate(iso)} ${formatTime(iso)}`;

/** JST の "YYYY-MM-DD"。 */
export const ymdJst = (d: Date = new Date()): string => ymdFmt.format(d);

/** "YYYY-MM-DD" に日数を足す（暦日の計算なのでタイムゾーンの影響を受けない）。 */
export function addDaysYmd(ymd: string, days: number): string {
  const [y, m, d] = ymd.split('-').map(Number) as [number, number, number];
  const t = new Date(Date.UTC(y, m - 1, d + days));
  return t.toISOString().slice(0, 10);
}

/** "YYYY-MM-DD" の曜日ラベル。 */
export function weekdayLabel(ymd: string): string {
  const [y, m, d] = ymd.split('-').map(Number) as [number, number, number];
  return ['日', '月', '火', '水', '木', '金', '土'][new Date(Date.UTC(y, m - 1, d)).getUTCDay()] ?? '';
}

export const WEEKDAYS = ['日', '月', '火', '水', '木', '金', '土'] as const;

/** 分を「約35分」「約1時間」「約1時間30分」に整形する。 */
export function formatWaitMinutes(minutes: number): string {
  if (minutes < 60) return `約${minutes}分`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m === 0 ? `約${h}時間` : `約${h}時間${m}分`;
}
