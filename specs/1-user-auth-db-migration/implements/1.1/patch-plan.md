---
kairo:
  node_id: "impl:1-user-auth-db-migration:1.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "1-user-auth-db-migration"
  task_id: "1.1"
  created_at: "2026-10-10T16:16:04+09:00"
coherence:
  id: "impl:1-user-auth-db-migration:1.1"
  depends_on:
    - id: "imp:1-user-auth-db-migration"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 1-user-auth-db-migration
task_id: 1.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T16:16:04+09:00

# パッチ計画: 1-user-auth-db-migration / 1.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/db/migrations/000001_create_users.{up,down}.sql | 新規 | users（role CHECK, email UNIQUE） | REQ-001-AC-1 |
| backend/db/migrations/000002_create_refresh_tokens.{up,down}.sql | 新規 | refresh_tokens + 2 インデックス | REQ-003-AC-3 |
| backend/db/migrations/000003_create_password_reset_tokens.{up,down}.sql | 新規 | password_reset_tokens + UNIQUE | REQ-004-AC-1 |
| backend/db/tests/migrations_test.sh | 新規 | up/down/再 up と制約の検証 | 全 AC |
| .github/workflows/ci.yml | 新規 | PostgreSQL 15 で上記テストを実行 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| backend/db/tests/migrations_test.sh | REQ-001-AC-1, REQ-003-AC-3, REQ-004-AC-1 | TEST_DATABASE_URL=postgres://…/booking_test backend/db/tests/migrations_test.sh |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] TDD Red フェーズのテストが全て通過した（pass=32 fail=0、PostgreSQL 16 で確認）
- [x] 既存テストが破壊されていない（既存テストなし）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（ci.yml は新規作成）
- [ ] golang-migrate CLI での実行確認（未導入のため未実施。CI で確認）
- [ ] PostgreSQL 15 での確認（CI で確認。ローカルは 16）
