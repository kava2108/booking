# Issue #3 — [user-auth] Go プロジェクト基盤 / 設定 / ミドルウェア

## メタデータ

| 項目 | 値 |
|---|---|
| Issue URL | https://github.com/kava2108/booking/issues/3 |
| Feature | user-auth |
| Task ID | 1.3 |
| Wave | P0 |
| Status | open |

## 対応スペックリンク

- Requirements: `.kiro/specs/user-auth/requirements.md` — REQ-002-AC-3
- Design: `.kiro/specs/user-auth/design.md` §セキュリティ
- Tech: `.kiro/steering/tech.md` §zerolog, §ulule/limiter
- Structure: `.kiro/steering/structure.md` §ディレクトリ構造
- Tasks: `.kiro/specs/user-auth/tasks.md` §1.3

## 依存関係

- **depends on**: #1 (DB マイグレーション — pgxpool 接続)
- middleware 単体は #1 なしで先行開発可能
- **被依存**: P1 タスク 3.1, 3.2, 4.1

## 成果物

```
go.mod / go.sum
cmd/api/main.go
internal/config/config.go
internal/middleware/
  request_id.go
  logger.go
  rate_limiter.go
```

## 完了条件

- [ ] `go build ./...` が通過する
- [ ] `go test ./internal/middleware/...` が通過する
- [ ] zerolog が request_id フィールドを含む JSON ログを出力する
- [ ] レートリミッター: 同一 IP から 5 回ログイン失敗後に 429 が返る
