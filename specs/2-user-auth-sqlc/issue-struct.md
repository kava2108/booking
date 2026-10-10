# Issue #2 — [user-auth] sqlc 設定と SQL クエリファイルの作成

## メタデータ

| 項目 | 値 |
|---|---|
| Issue URL | https://github.com/kava2108/booking/issues/2 |
| Feature | user-auth |
| Task ID | 1.2 |
| Wave | P0 |
| Status | open |

## 対応スペックリンク

- Requirements: `.kiro/specs/user-auth/requirements.md` — REQ-001〜005 各 AC-1
- Design: `.kiro/specs/user-auth/design.md` §APIデザイン, §DB設計
- Tech: `.kiro/steering/tech.md` §sqlc
- Tasks: `.kiro/specs/user-auth/tasks.md` §1.2

## 依存関係

- **depends on**: #1 (DB マイグレーション)
- **被依存**: P1 タスク 2.1 (Domain+Repository)

## 成果物

```
sqlc.yaml
db/query/
  user.sql     # CreateUser, GetUserByEmail, GetUserByID
  token.sql    # CreateRefreshToken, GetRefreshTokenByHash, RevokeRefreshToken
               # CreatePasswordResetToken, GetPasswordResetTokenByHash, MarkPasswordResetTokenUsed
internal/db/   # sqlc generate 出力 (Querier interface + 実装)
```

## 完了条件

- [ ] `sqlc generate` がエラーなく通過する
- [ ] `Querier` インターフェースが生成される
- [ ] 全 CRUD 操作に対応するメソッドが揃っている
