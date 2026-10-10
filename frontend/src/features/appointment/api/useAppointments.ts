import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { Appointment, AppointmentSummary, CreateAppointmentRequest } from '@/api/types';
import { useAuth } from '@/store/auth';

/** 自分の予約一覧（症状メモを含むため本人のみ）。 */
export function useMyAppointments() {
  const { status } = useAuth();
  return useQuery({
    queryKey: ['appointments', 'me'],
    queryFn: async () => (await api.get<AppointmentSummary[]>('/appointments/me')).data,
    enabled: status === 'authenticated',
  });
}

/** 予約の変更はいずれも、枠の空き状況と自分の予約一覧に影響する。 */
function useInvalidateBookingData() {
  const qc = useQueryClient();
  return () => Promise.all([qc.invalidateQueries({ queryKey: ['slots'] }), qc.invalidateQueries({ queryKey: ['appointments'] }), qc.invalidateQueries({ queryKey: ['position'] })]);
}

export function useCreateAppointment() {
  const invalidate = useInvalidateBookingData();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateAppointmentRequest) => (await api.post<Appointment>('/appointments', input)).data,
    onSuccess: invalidate,
    // 満席などで失敗したときも、枠の一覧を最新にして選び直せるようにする
    onError: () => qc.invalidateQueries({ queryKey: ['slots'] }),
  });
}

export function useCancelAppointment() {
  const invalidate = useInvalidateBookingData();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.delete(`/appointments/${id}`);
    },
    onSuccess: invalidate,
  });
}

export function useRescheduleAppointment() {
  const invalidate = useInvalidateBookingData();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, slotId }: { id: string; slotId: string }) => (await api.patch<Appointment>(`/appointments/${id}`, { slot_id: slotId })).data,
    onSuccess: invalidate,
    onError: () => qc.invalidateQueries({ queryKey: ['slots'] }),
  });
}
