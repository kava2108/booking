import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/api/client';
import { authStore } from '@/store/auth';

/** ログアウト。サーバー側の失効に失敗しても、端末側のログイン状態とキャッシュは必ず破棄する（共有端末対策）。 */
export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.post('/auth/logout');
    },
    onSettled: () => {
      authStore.clear();
      qc.clear();
    },
  });
}
