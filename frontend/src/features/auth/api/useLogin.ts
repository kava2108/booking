import { useMutation } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { LoginRequest, TokenResponse } from '@/api/types';
import { authStore } from '@/store/auth';

/** ログイン。アクセストークンはメモリに保持し、リフレッシュトークンは HttpOnly Cookie に入る。 */
export function useLogin() {
  return useMutation({
    mutationFn: async (input: LoginRequest) => (await api.post<TokenResponse>('/auth/login', input)).data,
    onSuccess: (data) => authStore.setToken(data.access_token),
  });
}
