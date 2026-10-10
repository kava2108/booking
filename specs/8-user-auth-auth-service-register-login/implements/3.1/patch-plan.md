---
kairo:
  node_id: "impl:8-user-auth-auth-service-register-login:3.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "8-user-auth-auth-service-register-login"
  task_id: "3.1"
  created_at: "2026-10-10T17:14:20+09:00"
coherence:
  id: "impl:8-user-auth-auth-service-register-login:3.1"
  depends_on:
    - id: "imp:8-user-auth-auth-service-register-login"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 8-user-auth-auth-service-register-login
task_id: 3.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T17:14:20+09:00

# パッチ計画: 8-user-auth-auth-service-register-login / 3.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/auth/token.go | 新規 | TokenManager（HS256 固定、exp/issuer/sub/role 検証）、opaque トークンと SHA-256 | REQ-002 |
| backend/internal/service/auth_service.go | 新規 | AuthService、Register / Login、エラー型 | REQ-001, 002 |
| backend/internal/service/mailer.go | 新規 | Mailer、SMTPMailer、NoopMailer | REQ-001-AC-1 |
| backend/internal/auth/token_test.go, service/auth_service_test.go, mailer_test.go | 新規 | ユニットテスト | 全 AC |
| backend/go.mod, go.sum | 変更 | golang-jwt/v5, x/crypto | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| auth/token_test.go, service/auth_service_test.go, service/mailer_test.go | REQ-001-AC-1/2/3, REQ-002-AC-1/2/4 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（auth 94.6%、service 92.8%）
- [x] 既存テストが破壊されていない
- [x] IMP の変更ファイル一覧と実際の変更が一致している
