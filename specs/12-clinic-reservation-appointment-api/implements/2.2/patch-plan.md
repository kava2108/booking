---
kairo:
  node_id: "impl:12-clinic-reservation-appointment-api:2.2"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "12-clinic-reservation-appointment-api"
  task_id: "2.2"
  created_at: "2026-10-10T18:14:55+09:00"
coherence:
  id: "impl:12-clinic-reservation-appointment-api:2.2"
  depends_on:
    - id: "imp:12-clinic-reservation-appointment-api"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 12-clinic-reservation-appointment-api
task_id: 2.2
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T18:14:55+09:00

# パッチ計画: 12-clinic-reservation-appointment-api / 2.2

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/service/appointment_service.go | 新規 | Create / ListMine / Cancel / Reschedule（同一 Tx・通知はアウトボックス） | REQ-002, 003 |
| backend/internal/domain/notification.go | 新規 | 通知種別と AppointmentNotice（症状メモのフィールドを持たない） | REQ-007 |
| backend/internal/handler/appointment_handler.go | 新規 | 4 エンドポイント、エラー変換 | — |
| backend/db/query/slot.sql | 変更 | GetSlotDetail（診療科名・医師名つき） | — |
| backend/internal/repository/errors.go | 変更 | 一意制約名の定数と UniqueViolationConstraint | — |
| backend/internal/app/app.go | 変更 | ルート結線 | — |
| backend/internal/app/appointments_integration_test.go | 新規 | HTTP 統合・並行・原子性・IDOR | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/appointment_service_test.go, app/appointments_integration_test.go, domain/notification_test.go, repository/errors_test.go | REQ-002-AC-1〜5, REQ-003-AC-1〜4, REQ-007-AC-2 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race。20 並行で定員超過なし）
- [x] 既存テストが破壊されていない
- [x] 実プロセスで確認（予約確定・二重予約 409）
- [x] IMP の変更ファイル一覧と実際の変更が一致している
