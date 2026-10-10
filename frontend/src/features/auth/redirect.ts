import type { Role } from '@/api/types';

/** ログイン後の既定の遷移先（ロール別）。 */
export function landingFor(role: Role | null): string {
  return role === 'staff' || role === 'clinic_admin' ? '/staff' : '/mypage';
}

/**
 * ログイン前に開こうとしたページへ戻るための検証。
 * アプリ内の絶対パス（"/" で始まり "//" や "\\" で始まらない）だけを許し、外部サイトへのリダイレクト（オープンリダイレクト）を防ぐ。
 */
export function safeRedirect(from: unknown): string | null {
  if (typeof from !== 'string') return null;
  if (!from.startsWith('/') || from.startsWith('//') || from.includes('\\')) return null;
  if (/^\/(login|register|password-reset)/.test(from)) return null;
  return from;
}
