---
kairo:
  node_id: "impl:16-clinic-reservation-closures:3.2"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "16-clinic-reservation-closures"
  task_id: "3.2"
  created_at: "2026-10-10T18:42:51+09:00"
coherence:
  id: "impl:16-clinic-reservation-closures:3.2"
  depends_on:
    - id: "imp:16-clinic-reservation-closures"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 16-clinic-reservation-closures
task_id: 3.2
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T18:42:51+09:00

# パッチ計画: 16-clinic-reservation-closures / 3.2

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/service/closure_service.go | 新規 | 検証・休診作成・枠の closed 化・通知登録・監査（同一 Tx） | REQ-005-AC-3 |
| backend/internal/handler/closure_handler.go | 新規 | POST /staff/closures | — |
| backend/internal/domain/notification.go | 変更 | clinic_closure 種別 | — |
| backend/internal/service/notification_templates.go | 変更 | 休診のお知らせ（定型文のみ） | REQ-007 |
| backend/db/query/appointment.sql | 変更 | ListBookedAppointmentsForNotice に枠 ID・診療科名・医師名を追加 | — |
| backend/internal/app/app.go, cmd/api/main.go | 変更 | 結線 | — |
| backend/internal/app/closures_integration_test.go | 新規 | HTTP 統合テスト | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/closure_service_test.go, notification_templates_test.go, app/closures_integration_test.go | REQ-005-AC-3/4, REQ-007 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race）
- [x] 既存テストが破壊されていない
- [x] 実プロセスで確認（休診 → 11 枠閉鎖・1 件通知・待ち時間 closed）
- [x] IMP の変更ファイル一覧と実際の変更が一致している
