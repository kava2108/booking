import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { WaitTimesResponse } from '@/api/types';

/** 待ち時間の自動更新間隔（REQ-004-AC-3: 30 秒以内）。 */
export const WAIT_TIME_POLL_MS = 30_000;

/**
 * 診療科別の待ち時間（未ログインでも取得できる公開 API）。
 * 更新に失敗しても react-query が前回のデータを保持するので、画面は前回値 + 「更新できなかった」を表示できる。
 */
export function useWaitTimes() {
  return useQuery({
    queryKey: ['waitTimes'],
    queryFn: async () => (await api.get<WaitTimesResponse>('/wait-times')).data,
    refetchInterval: WAIT_TIME_POLL_MS,
  });
}
