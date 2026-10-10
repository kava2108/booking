---
issue_id: 2-user-auth-sqlc
task_id: 1.2
created_at: 2026-10-10T16:39:54+09:00
---

# 実装判断メモ: 2-user-auth-sqlc / 1.2

## 採用したアプローチ

- クエリ名は issue-struct.md の表記（Get*/Create*/Revoke*/Mark*）に合わせた。IMP の変更ファイル一覧の Find* 表記は issue-struct と不一致だったため issue-struct を正とした。repository 層（P1 2.1）で Find* にラップする。
- `RevokeRefreshToken` / `MarkPasswordResetTokenUsed` は `WHERE … IS NULL` 付き。二重実行しても最初の時刻が残る（リプレイ検知のため）。
- トークン取得は失効・期限切れも返し、判定は service 層に任せる（失効済みトークンの再利用検知ができる）。
- sqlc は golang-migrate の `.down.sql` を無視して `db/migrations` を読む。#4 のテーブルも `models.go` に生成される（#5 でクエリ追加）。
- sqlc は docker イメージ `sqlc/sqlc:1.27.0` を使用（ホスト未導入）。CI も同じイメージで差分チェック。
- 生成物はコミット対象にし、CI で `git diff --exit-code` により鮮度を担保。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| 生成物を CI 生成のみにしてコミットしない | ローカルビルドに sqlc が必須になる |
| `SELECT *` を避け列挙 | users に列が増えるたび全クエリを直す必要がある。RETURNING * と合わせ生成型の一貫性を優先 |

## 既存コードへの影響

#1 のマイグレーションを変更しない。テストは `public` スキーマを再作成するため、マイグレーションの shell テストと同じ DB に**並行実行しない**こと（CI は別ジョブ・別 DB）。

## TODO・未解決事項

- [ ] #5 で clinic 用クエリを追加（別ファイル名: slot.sql 等）
- [ ] P1 2.1 で Querier を包む repository interface と mockery モックを作る
- [ ] `SELECT *` の password_hash が公開 DTO に漏れないよう repository/handler で型を分ける
