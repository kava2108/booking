---
kairo:
  node_id: "impl:11-clinic-reservation-slot-api-admin-settings:2.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "11-clinic-reservation-slot-api-admin-settings"
  task_id: "2.1"
  created_at: "2026-10-10T18:14:55+09:00"
coherence:
  id: "impl:11-clinic-reservation-slot-api-admin-settings:2.1"
  depends_on:
    - id: "imp:11-clinic-reservation-slot-api-admin-settings"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 11-clinic-reservation-slot-api-admin-settings
task_id: 2.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T18:14:55+09:00

# パッチ計画: 11-clinic-reservation-slot-api-admin-settings / 2.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/domain/clock.go | 新規 | JST（固定オフセット）、日付境界、ParseDate / ParseClock | — |
| backend/internal/service/slot_service.go | 新規 | 枠検索（理由つき）、冪等な枠生成、休診の再適用 | REQ-001, 006-AC-1 |
| backend/internal/service/admin_service.go | 新規 | スケジュール置換・設定更新（検証・Tx） | REQ-006 |
| backend/internal/service/slot_generator.go | 新規 | 定期生成ジョブ | REQ-006-AC-1 |
| backend/internal/service/clinic_common.go | 新規 | 共通オプション・ValidationError | — |
| backend/internal/handler/{catalog,admin,common}_handler.go | 新規 | 公開カタログ / 管理者 API / 共通ヘルパー | — |
| backend/internal/handler/validator.go | 変更 | ネストしたフィールドのパス（schedules[0].weekday） | — |
| backend/db/query/admin.sql | 変更 | GetDepartment, GetDoctor, ListClosuresFrom | — |
| backend/api/openapi.yaml, frontend/src/api/schema.d.ts | 変更 | outside_window / 400 / 500 の定義 | — |
| backend/internal/app/app.go, cmd/api/main.go | 変更 | 結線・日次ジョブ | — |
| backend/internal/app/clinic_env_test.go ほか | 新規 | HTTP 統合テストの環境（実 DB・固定時計・契約チェック） | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/slot_service_test.go, admin_service_test.go, app/catalog_admin_integration_test.go, domain/clock_test.go | REQ-001-AC-1/2/3, REQ-006-AC-1/2 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race）
- [x] 既存テストが破壊されていない
- [x] 実プロセスで起動し curl で確認（スケジュール設定 → 枠生成 → 公開検索）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（OpenAPI 追記・clock.go は差分）
