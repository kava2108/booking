import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { ClosureRequest, ClosureResult, QueueState, StaffAppointment, StaffQueueEntry, WalkInTicket } from '@/api/types';
import { useAuth } from '@/store/auth';

/** スタッフ画面の自動更新間隔。受付の現場なので、公開の待ち時間(30 秒)より短くする。 */
export const STAFF_POLL_MS = 15_000;

export function useStaffQueue(departmentId: number) {
  const { status } = useAuth();
  return useQuery({
    queryKey: ['staffQueue', departmentId],
    queryFn: async () => (await api.get<StaffQueueEntry[]>('/staff/queue', { params: { department_id: departmentId } })).data,
    refetchInterval: STAFF_POLL_MS,
    enabled: status === 'authenticated',
  });
}

export function useStaffAppointments(departmentId: number, date: string) {
  const { status } = useAuth();
  return useQuery({
    queryKey: ['staffAppointments', departmentId, date],
    queryFn: async () => (await api.get<StaffAppointment[]>('/staff/appointments', { params: { department_id: departmentId, date } })).data,
    refetchInterval: STAFF_POLL_MS,
    enabled: status === 'authenticated',
  });
}

/** 受付・待機列の操作は、待機列・当日予約・公開の待ち時間のすべてに影響する。 */
function useInvalidateOperations() {
  const qc = useQueryClient();
  return () => Promise.all(['staffQueue', 'staffAppointments', 'waitTimes'].map((k) => qc.invalidateQueries({ queryKey: [k] })));
}

export type QueueAction = 'call' | 'start' | 'complete';

/** 呼出 / 診察開始 / 診察完了。失敗（他の端末が先に操作した場合の 409 など）でも最新の状態に更新する。 */
export function useQueueAction() {
  const invalidate = useInvalidateOperations();
  return useMutation({
    mutationFn: async ({ id, action }: { id: string; action: QueueAction }) => (await api.post<QueueState>(`/staff/queue/${id}/${action}`)).data,
    onSettled: invalidate,
  });
}

/** 予約患者の来院受付。id は予約 ID。 */
export function useCheckIn() {
  const invalidate = useInvalidateOperations();
  return useMutation({
    mutationFn: async (appointmentId: string) => (await api.post<QueueState>(`/staff/queue/${appointmentId}/check-in`)).data,
    onSettled: invalidate,
  });
}

/** 当日受付の代行（アカウントのない患者）。 */
export function useStaffWalkIn() {
  const invalidate = useInvalidateOperations();
  return useMutation({
    mutationFn: async (input: { departmentId: number; displayName: string }) =>
      (await api.post<WalkInTicket>('/staff/walk-ins', { department_id: input.departmentId, display_name: input.displayName })).data,
    onSuccess: invalidate,
  });
}

export function useCreateClosure() {
  const qc = useQueryClient();
  const invalidate = useInvalidateOperations();
  return useMutation({
    mutationFn: async (input: ClosureRequest) => (await api.post<ClosureResult>('/staff/closures', input)).data,
    onSuccess: () => Promise.all([invalidate(), qc.invalidateQueries({ queryKey: ['slots'] })]),
  });
}
