---
kairo:
  node_id: "impl:13-clinic-reservation-queue-staff-walkin:2.3"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "13-clinic-reservation-queue-staff-walkin"
  task_id: "2.3"
  created_at: "2026-10-10T18:14:55+09:00"
coherence:
  id: "impl:13-clinic-reservation-queue-staff-walkin:2.3"
  depends_on:
    - id: "imp:13-clinic-reservation-queue-staff-walkin"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 13-clinic-reservation-queue-staff-walkin
task_id: 2.3
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T18:14:55+09:00

# パッチ計画: 13-clinic-reservation-queue-staff-walkin / 2.3

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/service/queue_service.go | 新規 | CheckIn / Call / Start / Complete / CreateWalkIn / ListStaffQueue | REQ-005, 008 |
| backend/internal/handler/queue_handler.go | 新規 | 患者の当日受付、staff 配下 6 エンドポイント、パラメータ検証ミドルウェア | — |
| backend/db/query/appointment.sql | 変更 | GetAppointmentWithSlot, CompleteAppointment | — |
| backend/db/query/queue.sql | 変更 | CountWalkInsForDate, CountActiveQueueEntriesForPatient | — |
| backend/db/query/admin.sql | 変更 | LockDepartment（FOR UPDATE） | REQ-008-AC-3 |
| backend/internal/repository/{repository,errors}.go | 変更 | interface と制約名の追加 | — |
| backend/api/openapi.yaml, frontend/src/api/schema.d.ts | 変更 | 409 / 422 の追記 | — |
| backend/internal/app/app.go | 変更 | StaffOnly + 監査の結線 | — |
| backend/internal/app/queue_integration_test.go | 新規 | HTTP 統合・並行・認可・監査 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/queue_service_test.go, app/queue_integration_test.go, repository/db/clinic_queries_test.go | REQ-005-AC-1/2/4, REQ-008-AC-1/2/3, REQ-007-AC-2/3 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race。並行当日受付で定員・採番が守られる）
- [x] 既存テストが破壊されていない
- [x] 実プロセスで確認（受付 → 呼出 → 開始 → 完了、予約が completed）
- [x] IMP の変更ファイル一覧と実際の変更が一致している
