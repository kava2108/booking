---
kairo:
  node_id: "impl:5-clinic-reservation-sqlc-repository:1.2"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "5-clinic-reservation-sqlc-repository"
  task_id: "1.2"
  created_at: "2026-10-10T16:49:22+09:00"
coherence:
  id: "impl:5-clinic-reservation-sqlc-repository:1.2"
  depends_on:
    - id: "imp:5-clinic-reservation-sqlc-repository"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 5-clinic-reservation-sqlc-repository
task_id: 1.2
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T16:49:22+09:00

# パッチ計画: 5-clinic-reservation-sqlc-repository / 1.2

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/db/query/slot.sql | 新規 | ListSlots, GetSlot, CreateSlotIfAbsent, ReserveSlot, ReleaseSlot, CloseSlots | REQ-002-AC-2 |
| backend/db/query/appointment.sql | 新規 | 作成/取得/一覧/キャンセル/変更/同時刻検出/休診通知対象 | REQ-002-AC-2 |
| backend/db/query/queue.sql | 新規 | 採番・状態遷移・CountAhead・所要時間・診察医数・スタッフ一覧 | REQ-004-AC-2 |
| backend/db/query/notification.sql | 新規 | Enqueue, ClaimNotifications（リース + SKIP LOCKED）, Mark* | — |
| backend/db/query/admin.sql | 新規 | マスタ・設定・スケジュール・休診・監査 | — |
| backend/internal/repository/db/*.sql.go, models.go, querier.go | 変更（生成物） | clinic クエリを追加 | — |
| backend/internal/repository/repository.go | 新規 | 5 つの repository interface、Repositories、TxRunner | — |
| backend/internal/repository/store.go | 新規 | pgxpool 上の Store（WithinTx）。*db.Queries をそのまま実装として利用 | REQ-002-AC-2 |
| backend/internal/repository/mocks/*.go, backend/.mockery.yaml | 新規（生成物） | mockery v2.53.5 | — |
| backend/internal/testutil/pgtest/pgtest.go | 新規 | テスト用: 専用スキーマ + マイグレーション適用 | — |
| backend/internal/repository/db/*_test.go, repository/*_test.go | 新規/変更 | 統合・Tx・モックのテスト。既存 auth テストは pgtest を使うよう変更（挙動同一） | 全 AC |
| .github/workflows/ci.yml | 変更 | mockery の差分チェックを追加 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| internal/repository/db/clinic_queries_test.go | REQ-002-AC-2, REQ-004-AC-2 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |
| internal/repository/store_test.go | REQ-002-AC-2 | 同上 |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] TDD Red フェーズのテストが全て通過した（-race、全パッケージ PASS。20 並行の確保で成功 1 件）
- [x] 既存テストが破壊されていない（#2 のテストも pgtest 化後に PASS）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（ListOpenSlots は ListSlots に変更。下記）
- [ ] PostgreSQL 15 での確認（CI）
