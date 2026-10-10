---
kairo:
  node_id: "impl:6-clinic-reservation-rbac-audit-openapi:1.3"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "6-clinic-reservation-rbac-audit-openapi"
  task_id: "1.3"
  created_at: "2026-10-10T16:55:06+09:00"
coherence:
  id: "impl:6-clinic-reservation-rbac-audit-openapi:1.3"
  depends_on:
    - id: "imp:6-clinic-reservation-rbac-audit-openapi"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 6-clinic-reservation-rbac-audit-openapi
task_id: 1.3
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T16:55:06+09:00

# パッチ計画: 6-clinic-reservation-rbac-audit-openapi / 1.3

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/internal/domain/role.go | 新規 | Role 型と 4 定数、Valid() | REQ-005-AC-4 |
| backend/internal/middleware/identity.go | 新規 | Identity と SetIdentity / IdentityFrom（user-auth の jwt_auth が設定する） | — |
| backend/internal/middleware/role.go | 新規 | RequireRole / StaffOnly / AdminOnly | REQ-005-AC-4 |
| backend/internal/middleware/audit.go | 新規 | AuditLog（ハンドラー前に記録、失敗時フェイルクローズ） | REQ-007-AC-3 |
| backend/internal/config/config.go | 変更 | RATE_LIMIT_PUBLIC_MAX/WINDOW を追加（既定 60 / 1m） | — |
| backend/api/openapi.yaml | 新規 | 19 エンドポイント。x-required-roles、security、エラー応答 | REQ-005-AC-4, REQ-007-AC-1/2 |
| backend/api/openapi_test.go | 新規 | 契約テスト（kin-openapi） | 同上 |
| frontend/package.json, tsconfig.json, package-lock.json | 新規 | openapi-typescript 7 / TypeScript 5 | — |
| frontend/src/api/schema.d.ts | 新規（生成物） | npm run gen:api の出力。手編集禁止 | — |
| frontend/src/api/types.ts, schema.check.ts | 新規 | 生成型の別名 / 型レベル契約チェック | REQ-007-AC-1 |
| .github/workflows/ci.yml | 変更 | 生成型の差分チェックと typecheck を追加 | — |
| .kiro/specs/user-auth/tasks.md | 変更 | Role の定義場所が domain/role.go になった旨を追記 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| internal/middleware/role_test.go | REQ-005-AC-4 | go test -race ./internal/middleware/... |
| internal/middleware/audit_test.go | REQ-007-AC-3 | 同上 |
| api/openapi_test.go | REQ-005-AC-4, REQ-007-AC-1/2 | go test ./api/... |
| frontend/src/api/schema.check.ts | REQ-007-AC-1 | cd frontend && npm run typecheck |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] TDD Red フェーズのテストが全て通過した（-race。middleware 98.5%、domain 100%）
- [x] 既存テストが破壊されていない（全パッケージ PASS）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（identity.go, domain/role.go, frontend 一式は差分）
- [ ] ルーターへの実結線（各ハンドラーは P1 以降。本 Issue はミドルウェアと契約まで）
- [ ] 公開 API 用レートリミットの適用（#3 の NewRateLimiter で別インスタンスを作る処理はルーティング時）
