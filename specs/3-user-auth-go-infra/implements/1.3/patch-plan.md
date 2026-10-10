---
kairo:
  node_id: "impl:3-user-auth-go-infra:1.3"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "3-user-auth-go-infra"
  task_id: "1.3"
  created_at: "2026-10-10T16:39:54+09:00"
coherence:
  id: "impl:3-user-auth-go-infra:1.3"
  depends_on:
    - id: "imp:3-user-auth-go-infra"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 3-user-auth-go-infra
task_id: 1.3
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T16:39:54+09:00

# パッチ計画: 3-user-auth-go-infra / 1.3

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/go.mod, go.sum | 新規 | module github.com/kava2108/booking/backend、Go 1.25 | — |
| backend/cmd/api/main.go | 新規 | newEcho（request_id→logger→recover、/healthz、ExtractIPDirect）と graceful shutdown | — |
| backend/internal/config/config.go | 新規 | viper で環境変数を読込・検証（必須項目はデフォルト無し） | — |
| backend/internal/middleware/request_id.go | 新規 | X-Request-ID の引継ぎ/生成（安全な値のみ） | — |
| backend/internal/middleware/logger.go | 新規 | zerolog。request_id 必須、秘匿情報は出力しない | — |
| backend/internal/middleware/rate_limiter.go | 新規 | ulule/limiter。Rate と Store を注入可能。ストア障害は 503 | REQ-002-AC-3 |
| backend/internal/**/_test.go, cmd/api/main_test.go | 新規 | 各テスト | 全 AC |
| .gitignore | 新規 | .env を除外 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| internal/middleware/*_test.go, internal/config/config_test.go, cmd/api/main_test.go | REQ-002-AC-3 ほか | go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] TDD Red フェーズのテストが全て通過した（-race で PASS。config 100% / middleware 97.6% / main 32%）
- [x] 既存テストが破壊されていない
- [x] IMP の変更ファイル一覧と実際の変更が一致している
- [ ] 実プロセスでの起動確認（DB_DSN 等を与えて /healthz を叩く）— 未実施
