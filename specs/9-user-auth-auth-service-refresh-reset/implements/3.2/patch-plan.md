---
kairo:
  node_id: "impl:9-user-auth-auth-service-refresh-reset:3.2"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "9-user-auth-auth-service-refresh-reset"
  task_id: "3.2"
  created_at: "2026-10-10T17:14:20+09:00"
coherence:
  id: "impl:9-user-auth-auth-service-refresh-reset:3.2"
  depends_on:
    - id: "imp:9-user-auth-auth-service-refresh-reset"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 9-user-auth-auth-service-refresh-reset
task_id: 3.2
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T17:14:20+09:00

# パッチ計画: 9-user-auth-auth-service-refresh-reset / 3.2

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/service/auth_service.go | 変更 | 4 メソッドの実装 | REQ-003, 004, 005 |
| backend/internal/service/auth_service_test.go | 変更 | 追加テスト | 同上 |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| service/auth_service_test.go | REQ-003-AC-1/2/3, REQ-004-AC-1/2/3/4, REQ-005-AC-1/2 | TEST_DATABASE_URL=postgres://…/booking_test go test -race ./... |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] テストが全て通過した
- [x] 既存テストが破壊されていない
- [x] IMP の変更ファイル一覧と実際の変更が一致している
