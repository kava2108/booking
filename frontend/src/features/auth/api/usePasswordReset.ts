import { useMutation } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { MessageResponse, PasswordResetConfirmRequest, PasswordResetRequest } from '@/api/types';

export function usePasswordResetRequest() {
  return useMutation({
    mutationFn: async (input: PasswordResetRequest) => (await api.post<MessageResponse>('/auth/password-reset/request', input)).data,
  });
}

export function usePasswordResetConfirm() {
  return useMutation({
    mutationFn: async (input: PasswordResetConfirmRequest) => (await api.post<MessageResponse>('/auth/password-reset/confirm', input)).data,
  });
}
