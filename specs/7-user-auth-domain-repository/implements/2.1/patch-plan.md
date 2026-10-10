---
kairo:
  node_id: "impl:7-user-auth-domain-repository:2.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "7-user-auth-domain-repository"
  task_id: "2.1"
  created_at: "2026-10-10T17:14:20+09:00"
coherence:
  id: "impl:7-user-auth-domain-repository:2.1"
  depends_on:
    - id: "imp:7-user-auth-domain-repository"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 7-user-auth-domain-repository
task_id: 2.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T17:14:20+09:00

# パッチ計画: 7-user-auth-domain-repository / 2.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/domain/user.go | 新規 | User / RefreshToken / PasswordResetToken と IsUsable | REQ-003, 004 |
| backend/internal/repository/errors.go | 新規 | ErrNotFound / ErrEmailTaken | REQ-001-AC-2 |
| backend/internal/repository/user_repository.go | 新規 | UserRepository と db.Querier 実装（pgx エラー変換） | REQ-001, 002 |
| backend/internal/repository/token_repository.go | 新規 | TokenRepository（失効/使用済み化は更新有無を返す） | REQ-003, 004 |
| backend/db/query/token.sql | 変更 | Revoke / MarkUsed を :execrows に、RevokeAllRefreshTokensForUser を追加 | REQ-003-AC-3 |
| backend/internal/repository/db/*（生成物） | 変更 | 再生成 | — |
| backend/internal/repository/db/auth_queries_test.go | 変更 | 戻り値変更に追随 | — |
| backend/.mockery.yaml, mocks/* | 変更 | UserRepository / TokenRepository のモック | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| domain/user_test.go, repository/user_repository_test.go, token_repository_test.go | REQ-001〜004 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race。domain 100%、repository 85.7%）
- [x] 既存テストが破壊されていない（#2 のテストは :execrows に追随）
- [x] IMP の変更ファイル一覧と実際の変更が一致している
