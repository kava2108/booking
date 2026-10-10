---
issue_id: 7-user-auth-domain-repository
task_id: 2.1
created_at: 2026-10-10T17:14:20+09:00
---

# 実装判断メモ: 7-user-auth-domain-repository / 2.1

## 採用したアプローチ

- **db.Querier を直接受け取る実装**（`NewUserRepository(q db.Querier)`）。トランザクション内でも `*db.Queries`（Tx 版）を渡せば同じ実装が使える。
- **ドメイン型へ変換し、DB 固有エラーを変換**: pgx.ErrNoRows → ErrNotFound、23505 → ErrEmailTaken。service は pgx を import しない。
- **失効・使用済み化を :execrows に変更し (bool, error) を返す**: 「同時に 2 つのリクエストが同じトークンを使う」競合を service が検知できる（10 並行で成功 1 件を検証）。#2 の sqlc クエリ変更を伴う。
- **RevokeAllRefreshTokens を追加**: パスワードリセット後に既存セッションを失効させるため（tasks.md にない追加。セキュリティ上必要と判断）。
- **domain.User は PasswordHash を保持**: service の検証に必要。HTTP に出さないことは #10 の DTO とテスト（レスポンス/ログにハッシュが無い）で担保。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| sqlc 生成型を service に直接渡す | 層の分離（Steering）に反し、モックも困難 |
| 失効を :exec のまま、取得後に状態を再確認 | TOCTOU 競合が残る |

## 既存コードへの影響

#2 の token.sql（戻り値）とテストを変更。clinic 側の repository（#5）とは独立。

## TODO・未解決事項

- [ ] UpdatePassword は存在しないユーザーでもエラーにならない（呼び出し側が事前に存在確認済みのため許容）
- [ ] email の小文字化は service の責務。DB 側に CHECK (email = lower(email)) を追加するか検討

