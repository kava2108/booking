# Issue 構造定義: 12-clinic-reservation-appointment-api

## 概要
予約 API（確定・一覧・変更・キャンセル）

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#2.2
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-002-AC-1 | WHEN ログイン済み患者が空き枠・来院区分（初診/再診）・症状メモを送信する THEN システムは予約を確定し予約番号を発行する | Must |
| REQ-002-AC-2 | WHEN 同一枠に複数の予約が同時に送信された THEN システムは排他制御により定員を超える予約を確定せず、後続に HTTP 409 を返す | Must |
| REQ-002-AC-4 | 予約確定の応答は 3 秒以内に返す | Must |
| REQ-002-AC-5 | WHEN 患者が同一時間帯に既に別の予約を持つ THEN システムは重複予約を拒否する | Should |
| REQ-003-AC-1 | WHEN 患者がマイページを開く THEN システムは自分の予約一覧（今後・過去）を表示する | Must |
| REQ-003-AC-2 | WHEN 患者がキャンセル期限内に予約をキャンセルする THEN システムは枠を解放し通知を送信する | Must |
| REQ-003-AC-3 | WHEN キャンセル期限を過ぎている THEN システムはキャンセル不可とし、医療機関への連絡方法を案内する | Should |
| REQ-003-AC-4 | WHEN 患者が日時を変更する THEN システムは新枠の確保と旧枠の解放を原子的に行う | Should |
| REQ-007-AC-2 | 症状メモ等の要配慮個人情報は、本人と権限を持つスタッフ以外が参照できないようアクセス制御する | Must |

## タスク一覧
- AppointmentService: 単一 UPDATE による枠確保と予約挿入を同一トランザクションで実施、0 行なら 409
- 同一時間帯の重複予約拒否、確定応答 3 秒以内
- キャンセル期限の判定、枠の解放、変更時の新枠確保と旧枠解放の原子処理
- 自分の予約一覧（patient_id = JWT subject で絞込）
- 症状メモを本人・スタッフ以外に返さない DTO 分離
- 同時リクエストによる定員超過が起きない並行テスト

## 依存関係
- 先行タスク: #4, #5, #6
- 並列波形: P1

## GitHub Issue
- Issue 番号: #12
- Issue URL: https://github.com/kava2108/booking/issues/12
