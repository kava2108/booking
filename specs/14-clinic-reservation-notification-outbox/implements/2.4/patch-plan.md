---
kairo:
  node_id: "impl:14-clinic-reservation-notification-outbox:2.4"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "14-clinic-reservation-notification-outbox"
  task_id: "2.4"
  created_at: "2026-10-10T18:14:55+09:00"
coherence:
  id: "impl:14-clinic-reservation-notification-outbox:2.4"
  depends_on:
    - id: "imp:14-clinic-reservation-notification-outbox"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 14-clinic-reservation-notification-outbox
task_id: 2.4
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T18:14:55+09:00

# パッチ計画: 14-clinic-reservation-notification-outbox / 2.4

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/db/migrations/000009_notification_dedupe.{up,down}.sql | 新規 | リマインド/順番接近の部分ユニークインデックス | REQ-002-AC-6 |
| backend/db/query/{notification,appointment,queue}.sql | 変更 | EnqueueNotificationOnce, RescheduleNotification, ListAppointmentsNeedingReminder, ListQueueEntriesNeedingNearNotice | — |
| backend/internal/service/notification_templates.go | 新規 | 種別ごとの本文（JST、症状なし） | REQ-007-AC-4 |
| backend/internal/service/notification_worker.go | 新規 | Claim → 描画 → 送信 → 結果更新（バックオフ・最大 5 回） | REQ-002-AC-3 |
| backend/internal/service/reminder_scheduler.go | 新規 | 前日リマインド・順番接近の登録 | REQ-002-AC-6 |
| backend/internal/service/mailer.go | 変更 | SMTPMailer.Send（汎用） | — |
| backend/internal/domain/notification.go | 変更 | リマインド/順番接近の種別と payload | — |
| backend/cmd/api/main.go | 変更 | ワーカー・スケジューラー・全サービスの起動/停止 | — |
| backend/db/tests/clinic_migrations_test.sh | 変更 | 000009 の検証 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/notification_{templates,worker,integration}_test.go, reminder_scheduler_test.go, repository/db/clinic_queries_test.go, db/tests/clinic_migrations_test.sh | REQ-002-AC-3/6, REQ-003-AC-2, REQ-007-AC-4 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race。service 90.4%）
- [x] 既存テストが破壊されていない
- [x] 実プロセスで確認（SMTP 未設定ではワーカーを起動せず通知は pending で残る）
- [ ] 実際の SMTP サーバーへの送信は未確認（ユニットは送信関数を差し替え）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（000009 は新規マイグレーション）
