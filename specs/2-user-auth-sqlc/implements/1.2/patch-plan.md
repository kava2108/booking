---
kairo:
  node_id: "impl:2-user-auth-sqlc:1.2"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "2-user-auth-sqlc"
  task_id: "1.2"
  created_at: "2026-10-10T16:39:54+09:00"
coherence:
  id: "impl:2-user-auth-sqlc:1.2"
  depends_on:
    - id: "imp:2-user-auth-sqlc"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 2-user-auth-sqlc
task_id: 1.2
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T16:39:54+09:00

# パッチ計画: 2-user-auth-sqlc / 1.2

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/sqlc.yaml | 新規 | pgx/v5、emit_interface、uuid→google/uuid | — |
| backend/db/query/user.sql | 新規 | CreateUser / GetUserByEmail / GetUserByID / UpdateUserPassword | REQ-001-AC-1, REQ-002-AC-1 |
| backend/db/query/token.sql | 新規 | リフレッシュ・リセットトークンの作成/取得/失効 | REQ-003-AC-1, REQ-004-AC-1 |
| backend/internal/repository/db/*.go | 新規（生成物） | sqlc generate の出力（手編集禁止） | — |
| backend/internal/repository/db/auth_queries_test.go, helpers_test.go | 新規 | 生成クエリの統合テスト | 全 AC |
| backend/go.mod, go.sum | 変更 | pgx/v5, google/uuid を追加 | — |
| .github/workflows/ci.yml | 変更 | sqlc generate の差分チェック + Go テスト | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| backend/internal/repository/db/auth_queries_test.go | REQ-001-AC-1/2, REQ-002-AC-1, REQ-003-AC-1, REQ-004-AC-1 | TEST_DATABASE_URL=postgres://…/booking_test go test ./internal/repository/... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] TDD Red フェーズのテストが全て通過した（9 件 PASS、coverage 97.2%、PostgreSQL 16）
- [x] 既存テストが破壊されていない
- [x] IMP の変更ファイル一覧と実際の変更が一致している（go.mod/go.sum・テスト・ci.yml は差分）
- [ ] PostgreSQL 15 での確認（CI）
