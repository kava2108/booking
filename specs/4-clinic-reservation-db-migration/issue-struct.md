# Issue 構造定義: 4-clinic-reservation-db-migration

## 概要
DB マイグレーション作成

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#1.1
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-002-AC-2 | WHEN 同一枠に複数の予約が同時に送信された THEN システムは排他制御により定員を超える予約を確定せず、後続に HTTP 409 を返す | Must |
| REQ-005-AC-4 | スタッフ機能はロールベースで制限し、患者は利用できない | Must |
| REQ-007-AC-3 | 通信は TLS で暗号化し、個人情報へのアクセスを監査ログに記録する | Must |
| REQ-009-AC-1 | システムは電子カルテ・既存受付システムに依存せず、予約・待機列・診察実績を自システム内のみで管理する | Must |

## タスク一覧
- users.role は user-auth 000001 で定義済みのため変更しない
- departments, doctors, doctor_schedules, slots を作成（CHECK booked_count <= capacity、UNIQUE doctor_id+start_at）
- appointments（部分ユニーク patient_id+slot_id WHERE status='booked'）、queue_entries、closures、notifications、audit_logs を作成
- 設計書記載のインデックスを付与
- 全マイグレーションを up/down ペアで作成し、チェックサム検証を CI に含める

## 依存関係
- 先行タスク: なし（前提: user-auth 完了）
- 並列波形: P0

## GitHub Issue
- Issue 番号: #4
- Issue URL: https://github.com/kava2108/booking/issues/4
