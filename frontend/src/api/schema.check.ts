// コンパイル時の契約チェック（tsc --noEmit で検証。実行はしない）。
import type { CreateAppointmentRequest, DepartmentWaitTime, WaitTimesResponse } from './types';

// 設計書 §API Design の応答例
export const example: WaitTimesResponse = {
  updated_at: '2026-10-12T09:41:10+09:00',
  note: '待ち時間は目安です。混雑状況により変動します',
  departments: [
    { department_id: 1, name: '内科', status: 'available', waiting_count: 6, wait_minutes: 35, basis: 'actual' },
    { department_id: 2, name: '小児科', status: 'unavailable', reason: 'before_hours' },
  ],
};

export const request: CreateAppointmentRequest = { slot_id: '7b1c', visit_type: 'revisit', symptom_note: '3日前から咳' };

type Equals<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Expect<T extends true> = T;

// 公開 API の型に個人情報の項目が存在しない
type PublicKeys = keyof DepartmentWaitTime;
export type _NoPatientName = Expect<Equals<Extract<PublicKeys, 'patient_name' | 'symptom_note' | 'display_name'>, never>>;

// visit_type は first / revisit のみ（不正値はコンパイルエラー）
// @ts-expect-error 不正な visit_type はコンパイルエラーになる
export const bad: CreateAppointmentRequest = { slot_id: 'x', visit_type: 'other' };
