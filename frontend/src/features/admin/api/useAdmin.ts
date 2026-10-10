import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { Schedules, SchedulesRequest, Settings, SettingsRequest } from '@/api/types';
import { useAuth } from '@/store/auth';

/** 管理者画面用: 現在の設定（受付期間・キャンセル期限・診療科ごとの設定）。 */
export function useAdminSettings() {
  const { status } = useAuth();
  return useQuery({
    queryKey: ['adminSettings'],
    queryFn: async () => (await api.get<Settings>('/admin/settings')).data,
    enabled: status === 'authenticated',
  });
}

export function useUpdateSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: SettingsRequest) => (await api.put<Settings>('/admin/settings', input)).data,
    // 受付期間・キャンセル期限は、枠の表示可否・予約一覧の「キャンセル可」・待ち時間（標準診察時間）に影響する
    onSuccess: () => Promise.all(['adminSettings', 'slots', 'appointments', 'waitTimes'].map((k) => qc.invalidateQueries({ queryKey: [k] }))),
  });
}

export function useAdminSchedules(doctorId: number | undefined) {
  const { status } = useAuth();
  return useQuery({
    queryKey: ['adminSchedules', doctorId],
    queryFn: async () => (await api.get<Schedules>('/admin/schedules', { params: { doctor_id: doctorId } })).data,
    enabled: status === 'authenticated' && doctorId !== undefined,
  });
}

export function usePutSchedules() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: SchedulesRequest) => {
      await api.put('/admin/schedules', input);
    },
    onSuccess: () => Promise.all(['adminSchedules', 'slots'].map((k) => qc.invalidateQueries({ queryKey: [k] }))),
  });
}
