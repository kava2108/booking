# Red フェーズ: 1-user-auth-db-migration / 1.1

テスト: `backend/db/tests/migrations_test.sh`（psql ベース。Go 未導入のため SQL 検証は shell で実施）

マイグレーション未作成の状態で実行 → **pass=2 fail=28**（期待どおり失敗）。

| 観点 | ケース | AC |
|---|---|---|
| 構造 | up/down ペア、up 3 本以上、3 テーブル作成 | — |
| users | role 既定 patient / 許可 3 値 / 'user' は CHECK 違反 / email 重複 / password_hash 73 文字 / email NULL | REQ-001-AC-1 |
| refresh_tokens | 登録 / token_hash 重複 / FK 違反 / revoked_at 既定 NULL | REQ-003-AC-3 |
| password_reset_tokens | 登録 / token_hash 重複 / used_at 既定 NULL | REQ-004-AC-1 |
| インデックス | users(email), refresh_tokens(token_hash), (user_id, revoked_at), password_reset_tokens(token_hash) | — |
| 連鎖 | ユーザー削除でトークン削除 | — |
| 冪等 | down → 全削除 → 再 up | — |
