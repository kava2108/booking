import { useMutation } from '@tanstack/react-query';
import { api } from '@/api/client';
import type { RegisterRequest, RegisterResponse } from '@/api/types';

export function useRegister() {
  return useMutation({
    mutationFn: async (input: RegisterRequest) => (await api.post<RegisterResponse>('/auth/register', input)).data,
  });
}
