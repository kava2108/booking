# Issue 構造定義: 14-clinic-reservation-notification-outbox

## 概要
通知基盤（アウトボックス・メール・リマインド）

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#2.4
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-002-AC-3 | WHEN 予約が確定した THEN システムは確認通知（日時・診療科・予約番号）をメールで送信する | Must |
| REQ-002-AC-6 | WHEN 予約の前日と、自分の順番が近づいた時点（あと数人）に達した THEN システムはリマインド通知を送信する | Should |
| REQ-003-AC-2 | WHEN 患者がキャンセル期限内に予約をキャンセルする THEN システムは枠を解放し通知を送信する | Must |
| REQ-007-AC-4 | 個人情報保護法および医療情報システムの安全管理ガイドラインに準拠する | Must |

## タスク一覧
- Notifier interface と SMTP 実装、NotificationService（notifications へ登録）
- ワーカー: SKIP LOCKED で取得、指数バックオフで最大 5 回再試行
- 予約確認・キャンセル・前日リマインド・「あと N 人」通知の本文テンプレート（症状・診断は含めない）
- ReminderScheduler

## 依存関係
- 先行タスク: #4, #5
- 並列波形: P1

## GitHub Issue
- Issue 番号: #14
- Issue URL: https://github.com/kava2108/booking/issues/14
