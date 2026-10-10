---
kairo:
  node_id: "impl:10-user-auth-auth-handler:4.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "10-user-auth-auth-handler"
  task_id: "4.1"
  created_at: "2026-10-10T17:14:20+09:00"
coherence:
  id: "impl:10-user-auth-auth-handler:4.1"
  depends_on:
    - id: "imp:10-user-auth-auth-handler"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 10-user-auth-auth-handler
task_id: 4.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T17:14:20+09:00

# パッチ計画: 10-user-auth-auth-handler / 4.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/apierror/apierror.go | 新規 | 統一エラー形式と HTTPErrorHandler（5xx は汎用メッセージ） | — |
| backend/internal/handler/auth_handler.go | 新規 | 6 エンドポイント、Cookie、DTO、ルート登録 | REQ-001〜005 |
| backend/internal/handler/validator.go | 新規 | go-playground/validator の結線とフィールド別エラー | REQ-001-AC-3 |
| backend/internal/middleware/jwt_auth.go | 新規 | Bearer 検証 → Identity 設定 | REQ-002, clinic #6 |
| backend/internal/middleware/errors.go | 新規 | 認証系ミドルウェアの共通エラー応答 | — |
| backend/internal/middleware/{role,audit,logger}.go | 変更 | エラー形式の統一、5xx の原因ログ、監査失敗は error を返す | — |
| backend/internal/app/app.go | 新規 | ミドルウェアチェイン・ルート・レートリミッタの結線（main の newEcho を移動） | REQ-002-AC-3 |
| backend/cmd/api/main.go | 変更 | DB 接続・サービス結線・graceful shutdown | — |
| backend/cmd/api/main_test.go | 削除 | app/app_test.go に移動 | — |
| backend/internal/config/config.go | 変更 | COOKIE_SECURE、APP_BASE_URL | — |
| backend/go.mod, go.sum | 変更 | validator v10.30.0 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| app/auth_integration_test.go, app/app_test.go, middleware/jwt_auth_test.go, config/config_test.go | REQ-001〜005 全 AC | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した（-race。app 100%、handler/apierror/middleware 合算 81.5%）
- [x] 既存テストが破壊されていない（main_test は app_test へ移動）
- [x] 実プロセスで起動し curl で確認（healthz / register / login / refresh / 405。ログにパスワードなし）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（app/ apierror/ は差分）
