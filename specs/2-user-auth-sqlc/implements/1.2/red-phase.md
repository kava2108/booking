# Red フェーズ: 2-user-auth-sqlc / 1.2

テスト: `backend/internal/repository/db/auth_queries_test.go`（`TEST_DATABASE_URL` が無ければスキップ）

sqlc.yaml / クエリ未作成の状態 → `go vet` は依存不足、`sqlc generate` は「no queries contained」で失敗（期待どおり）。

| ケース | AC |
|---|---|
| CreateUser（role 既定 patient）/ GetUserByEmail / GetUserByID | REQ-001-AC-1, REQ-002-AC-1 |
| 重複 email は 23505 | REQ-001-AC-2 |
| 未登録 email は pgx.ErrNoRows | — |
| UpdateUserPassword（hash と updated_at 更新） | REQ-004 |
| リフレッシュトークン作成→取得→失効（失効後も行を返す） | REQ-003-AC-1 |
| 未知のハッシュは ErrNoRows | — |
| リセットトークン作成→取得→使用済み化 / 二重使用済み化で時刻が変わらない | REQ-004-AC-1 |
| Querier interface の網羅 | — |
