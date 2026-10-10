# Issue #1 — [user-auth] DB マイグレーション作成

## メタデータ

| 項目 | 値 |
|---|---|
| Issue URL | https://github.com/kava2108/booking/issues/1 |
| Feature | user-auth |
| Task ID | 1.1 |
| Wave | P0 |
| Status | open |

## 対応スペックリンク

- Requirements: `.kiro/specs/user-auth/requirements.md` — REQ-001-AC-1, REQ-003-AC-3, REQ-004-AC-1
- Design: `.kiro/specs/user-auth/design.md` §DB設計
- Tasks: `.kiro/specs/user-auth/tasks.md` §1.1

## 依存関係

- 依存なし（P0 先頭タスク）
- **被依存**: #2 (sqlc), #3 (Go 基盤)

## 成果物

```
db/migrations/
  000001_create_users.up.sql
  000001_create_users.down.sql
  000002_create_refresh_tokens.up.sql
  000002_create_refresh_tokens.down.sql
  000003_create_password_reset_tokens.up.sql
  000003_create_password_reset_tokens.down.sql
```

## 完了条件

- [ ] `migrate up` が冪等に実行できる
- [ ] `migrate down` で完全にロールバックできる
- [ ] CI で `migrate up` が通過する
