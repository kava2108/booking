import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';
import { parseApiError, userMessage } from './errors';

function axiosError(status: number, data: unknown, headers: Record<string, string> = {}): AxiosError {
  const response = { status, data, headers: new AxiosHeaders(headers), statusText: '', config: { headers: new AxiosHeaders() } } as AxiosResponse;
  return new AxiosError('failed', 'ERR_BAD_REQUEST', undefined, undefined, response);
}

describe('parseApiError', () => {
  it('統一エラー形式を読み取る', () => {
    const e = parseApiError(axiosError(409, { error: 'slot_full', message: 'm', request_id: 'req-1' }));

    expect(e).toMatchObject({ status: 409, code: 'slot_full', requestId: 'req-1' });
  });

  it('バリデーションエラーはフィールド別に取り出す', () => {
    const e = parseApiError(axiosError(422, { error: 'validation_failed', details: [{ field: 'password', message: 'must be at least 12 characters' }, { field: 'email', message: 'is required' }] }));

    expect(e.fields).toEqual({ password: 'must be at least 12 characters', email: 'is required' });
  });

  it('Retry-After を秒数で保持する', () => {
    const e = parseApiError(axiosError(429, { error: 'too_many_requests' }, { 'retry-after': '900' }));

    expect(e.retryAfterSeconds).toBe(900);
  });

  it('レスポンスが無い（通信失敗）は network エラー', () => {
    const e = parseApiError(new AxiosError('Network Error', 'ERR_NETWORK'));

    expect(e.code).toBe('network_error');
    expect(e.status).toBe(0);
  });

  it('JSON でない応答や未知のエラーでも落ちない', () => {
    expect(parseApiError(axiosError(502, '<html>Bad Gateway</html>')).code).toBe('unknown_error');
    expect(parseApiError(new Error('boom')).code).toBe('unknown_error');
    expect(parseApiError('string error').code).toBe('unknown_error');
  });
});

describe('userMessage（利用者向けの日本語メッセージ）', () => {
  const msg = (status: number, data: Record<string, unknown>, headers?: Record<string, string>) => userMessage(parseApiError(axiosError(status, data, headers)));

  it.each([
    [409, 'slot_full', '満席'],
    [409, 'duplicate_appointment', '同じ時間帯'],
    [422, 'cancel_deadline_passed', '期限'],
    [401, 'invalid_credentials', 'メールアドレスまたはパスワード'],
    [409, 'email_already_registered', '登録済み'],
    [400, 'invalid_reset_token', '有効期限'],
    [409, 'already_in_queue', '既に'],
    [409, 'invalid_transition', '状態'],
    [404, 'not_found', '見つかりません'],
    [403, 'forbidden', '権限'],
  ])('%i %s', (status, code, expected) => {
    expect(msg(status, { error: code })).toContain(expected);
  });

  it('当日受付の締切・定員は理由を伝える', () => {
    expect(msg(422, { error: 'walkin_closed', message: 'walk-in registration is closed (after the cutoff time)' })).toContain('受付時間');
    expect(msg(422, { error: 'walkin_closed', message: 'walk-in registration is closed (capacity reached)' })).toContain('定員');
  });

  it('レートリミットは待つ目安の分数を伝える', () => {
    expect(msg(429, { error: 'too_many_requests' }, { 'retry-after': '900' })).toContain('15分');
    expect(msg(429, { error: 'too_many_requests' })).toContain('しばらく');
  });

  it('サーバーエラーは問い合わせ番号を添える（内部情報は出さない）', () => {
    const m = msg(500, { error: 'internal_error', message: 'internal server error', request_id: 'req-9' });

    expect(m).toContain('req-9');
    expect(m).not.toContain('internal server error');
  });

  it('通信失敗は通信エラーとして案内する', () => {
    expect(userMessage(parseApiError(new AxiosError('Network Error', 'ERR_NETWORK')))).toContain('通信');
  });

  it('未知のコードは汎用メッセージ', () => {
    expect(msg(418, { error: 'teapot' })).toContain('エラー');
  });
});
