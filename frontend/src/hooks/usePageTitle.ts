import { useEffect } from 'react';

const SITE = 'クリニック予約';

/** 画面ごとにドキュメントタイトルを更新する（スクリーンリーダー・タブの識別用）。 */
export function usePageTitle(title: string): void {
  useEffect(() => {
    document.title = `${title} | ${SITE}`;
  }, [title]);
}
