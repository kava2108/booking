---
kairo:
  node_id: "impl:15-clinic-reservation-wait-time-api:3.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "15-clinic-reservation-wait-time-api"
  task_id: "3.1"
  created_at: "2026-10-10T18:42:51+09:00"
coherence:
  id: "impl:15-clinic-reservation-wait-time-api:3.1"
  depends_on:
    - id: "imp:15-clinic-reservation-wait-time-api"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 15-clinic-reservation-wait-time-api
task_id: 3.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T18:42:51+09:00

# パッチ計画: 15-clinic-reservation-wait-time-api / 3.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/service/wait_time_service.go | 新規 | 診療科別の算出・本人の順番・10 秒 TTL キャッシュ | REQ-004 |
| backend/internal/handler/waittime_handler.go | 新規 | GET /wait-times（公開）、GET /appointments/{id}/position | REQ-004, 007-AC-1 |
| backend/db/query/queue.sql | 変更 | CountScheduledDoctors, CountDoctorsStartingLater（休診と時間外の区別） | REQ-004-AC-5 |
| backend/internal/service/clinic_common.go | 変更 | pgUUID ヘルパー | — |
| backend/internal/app/app.go, cmd/api/main.go | 変更 | 結線 | — |
| backend/internal/app/waittime_integration_test.go | 新規 | HTTP 統合テスト | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/wait_time_service_test.go, app/waittime_integration_test.go, repository/db/clinic_queries_test.go | REQ-004-AC-1〜6, REQ-007-AC-1 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race）
- [x] 既存テストが破壊されていない
- [x] 実プロセスで確認（3 人待ち → 30 分、キャッシュ、他人の順番 404）
- [x] IMP の変更ファイル一覧と実際の変更が一致している
