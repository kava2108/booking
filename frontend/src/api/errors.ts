import { isAxiosError } from 'axios';

/** サーバーの統一エラー形式 {error, message, request_id, details} を扱いやすくしたもの。 */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    public readonly serverMessage: string,
    public readonly requestId: string | undefined,
    public readonly fields: Record<string, string>,
    public readonly retryAfterSeconds: number | undefined,
  ) {
    super(`${code}: ${serverMessage}`);
    this.name = 'ApiError';
  }
}

function header(headers: unknown, name: string): string | undefined {
  if (!headers) return undefined;
  const h = headers as { get?: (n: string) => unknown } & Record<string, unknown>;
  const v = typeof h.get === 'function' ? h.get(name) : h[name];
  return typeof v === 'string' ? v : undefined;
}

export function parseApiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err;
  if (!isAxiosError(err)) return new ApiError(0, 'unknown_error', err instanceof Error ? err.message : String(err), undefined, {}, undefined);
  if (!err.response) return new ApiError(0, 'network_error', err.message, undefined, {}, undefined);

  const { status, data, headers } = err.response;
  const body = (typeof data === 'object' && data !== null ? data : {}) as {
    error?: unknown; message?: unknown; request_id?: unknown; details?: unknown;
  };
  const fields: Record<string, string> = {};
  if (Array.isArray(body.details)) {
    for (const d of body.details as { field?: unknown; message?: unknown }[]) {
      if (typeof d.field === 'string' && typeof d.message === 'string') fields[d.field] = d.message;
    }
  }
  const retry = Number(header(headers, 'retry-after'));
  return new ApiError(
    status,
    typeof body.error === 'string' ? body.error : 'unknown_error',
    typeof body.message === 'string' ? body.message : '',
    typeof body.request_id === 'string' ? body.request_id : undefined,
    fields,
    Number.isFinite(retry) && retry > 0 ? retry : undefined,
  );
}

/** 利用者に見せる日本語のメッセージ。サーバーの内部メッセージは出さない（問い合わせ番号のみ添える）。 */
export function userMessage(e: ApiError): string {
  switch (e.code) {
    case 'network_error':
      return '通信に失敗しました。インターネット接続をご確認のうえ、もう一度お試しください。';
    case 'slot_full':
      return 'この枠は満席になりました。別の枠をお選びください。';
    case 'duplicate_appointment':
      return '同じ時間帯に既にご予約があります。';
    case 'cancel_deadline_passed':
      return 'キャンセル・変更の期限を過ぎています。お手数ですが医療機関へお電話ください。';
    case 'invalid_credentials':
      return 'メールアドレスまたはパスワードが正しくありません。';
    case 'email_already_registered':
      return 'このメールアドレスは既に登録済みです。ログインするか、パスワードをリセットしてください。';
    case 'invalid_reset_token':
      return 'リンクの有効期限が切れているか、既に使用されています。もう一度パスワードのリセットをお申し込みください。';
    case 'invalid_token':
      return 'ログインの有効期限が切れました。もう一度ログインしてください。';
    case 'already_in_queue':
      return '既に順番待ちに登録されています。';
    case 'invalid_transition':
      return '現在の状態ではこの操作はできません。画面を更新して最新の状態をご確認ください。';
    case 'walkin_closed':
      if (e.serverMessage.includes('cutoff')) return '本日の当日受付は受付時間を過ぎたため終了しました。';
      if (e.serverMessage.includes('capacity')) return '本日の当日受付は定員に達したため終了しました。';
      return '現在、当日受付はできません。';
    case 'not_found':
      return '対象が見つかりません。画面を更新してください。';
    case 'forbidden':
      return 'この操作を行う権限がありません。';
    case 'unauthorized':
      return 'ログインが必要です。';
    case 'payload_too_large':
      return '入力内容が大きすぎます。';
    case 'too_many_requests': {
      if (!e.retryAfterSeconds) return '操作が多すぎます。しばらくしてからやり直してください。';
      const min = Math.ceil(e.retryAfterSeconds / 60);
      return `操作が多すぎます。約${min}分後にやり直してください。`;
    }
    case 'validation_failed':
      return '入力内容をご確認ください。';
    case 'invalid_request':
      return 'リクエストの形式が正しくありません。';
  }
  if (e.status >= 500) return `サーバーでエラーが発生しました。時間をおいてやり直してください。${e.requestId ? `（問い合わせ番号: ${e.requestId}）` : ''}`;
  return '処理に失敗しました。エラーが続く場合は医療機関へご連絡ください。';
}
