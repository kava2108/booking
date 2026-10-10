import { z } from 'zod';

// バックエンドの検証（backend: 12 文字以上・72 バイト以下）と揃える。
const charCount = (s: string): number => [...s].length;
const byteLength = (s: string): number => new TextEncoder().encode(s).length;

const email = z.string().min(1, 'メールアドレスを入力してください').pipe(z.email('メールアドレスの形式が正しくありません'));

/** 新しいパスワード。12 文字以上、bcrypt の上限 72 バイト以内。 */
export const newPassword = z
  .string()
  .min(1, 'パスワードを入力してください')
  .refine((v) => charCount(v) >= 12, 'パスワードは12文字以上で入力してください')
  .refine((v) => byteLength(v) <= 72, 'パスワードが長すぎます');

export const loginSchema = z.object({
  email,
  password: z.string().min(1, 'パスワードを入力してください'),
});

export const registerSchema = z
  .object({
    name: z.string().trim().min(1, 'お名前を入力してください').max(100, 'お名前は100文字以内で入力してください'),
    email,
    password: newPassword,
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'パスワードが一致しません' });

export const resetRequestSchema = z.object({ email });

export const resetConfirmSchema = z
  .object({ password: newPassword, confirm: z.string() })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'パスワードが一致しません' });

export type LoginValues = z.infer<typeof loginSchema>;
export type RegisterValues = z.infer<typeof registerSchema>;
export type ResetRequestValues = z.infer<typeof resetRequestSchema>;
export type ResetConfirmValues = z.infer<typeof resetConfirmSchema>;

/** サーバーのフィールドエラー（英語）を日本語にする。 */
export function serverFieldMessage(message: string): string {
  if (message.includes('valid email')) return 'メールアドレスの形式が正しくありません';
  if (message.includes('at least 12') || message.includes('12 to 72')) return 'パスワードは12文字以上72バイト以内で入力してください';
  if (message.includes('required')) return '入力してください';
  return '入力内容を確認してください';
}
