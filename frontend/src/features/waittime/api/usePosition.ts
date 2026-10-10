import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { Position } from '@/api/types';
import { WAIT_TIME_POLL_MS } from './useWaitTimes';

/** 予約した本人の順番と呼出見込み。当日の来院後に 30 秒間隔で更新する。 */
export function usePosition(appointmentId: string) {
  return useQuery({
    queryKey: ['position', appointmentId],
    queryFn: async () => (await api.get<Position>(`/appointments/${appointmentId}/position`)).data,
    refetchInterval: WAIT_TIME_POLL_MS,
    retry: false,
  });
}
