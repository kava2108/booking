import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { WalkInTicket } from '@/api/types';

/** 患者本人の当日受付（予約なしで順番に並ぶ）。 */
export function useWalkIn() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (departmentId: number) => (await api.post<WalkInTicket>('/walk-ins', { department_id: departmentId })).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['waitTimes'] }),
  });
}
