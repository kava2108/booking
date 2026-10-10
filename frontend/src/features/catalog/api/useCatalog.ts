import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { Department, Doctor, Slot } from '@/api/types';

/** 診療科・医師・空き枠（認証不要の公開 API）。 */
export function useDepartments() {
  return useQuery({
    queryKey: ['departments'],
    queryFn: async () => (await api.get<Department[]>('/departments')).data,
    staleTime: 5 * 60_000, // マスタ情報なので長めにキャッシュ
  });
}

export function useDoctors(departmentId: number | undefined) {
  return useQuery({
    queryKey: ['doctors', departmentId],
    queryFn: async () => (await api.get<Doctor[]>('/doctors', { params: { department_id: departmentId } })).data,
    enabled: departmentId !== undefined,
    staleTime: 5 * 60_000,
  });
}

interface SlotParams {
  departmentId: number | undefined;
  doctorId?: number | undefined;
  date: string; // YYYY-MM-DD（JST）
}

export function useSlots({ departmentId, doctorId, date }: SlotParams) {
  return useQuery({
    queryKey: ['slots', departmentId, doctorId ?? null, date],
    queryFn: async () => (await api.get<Slot[]>('/slots', { params: { department_id: departmentId, doctor_id: doctorId, date } })).data,
    enabled: departmentId !== undefined,
  });
}
