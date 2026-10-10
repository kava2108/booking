---
issue_id: 1-user-auth-db-migration
task_id: 1.1
created_at: 2026-10-10T16:16:04+09:00
---

# 実装判断メモ: 1-user-auth-db-migration / 1.1

## 採用したアプローチ

- ID は `gen_random_uuid()`（PostgreSQL 13+ 標準）。設計書の `uuid_generate_v4()` は拡張が必要なため置換。
- `users.role` は varchar(20) + CHECK（patient / staff / clinic_admin / system_admin、既定 patient）。clinic-reservation と共有する値で、2026-10-10 に user-auth 設計書も更新済み。
- email の UNIQUE は `CREATE UNIQUE INDEX` で定義（設計書の「UNIQUE INDEX」に一致）。小文字化は service 層の責務。
- トークン系は `ON DELETE CASCADE`（ユーザー削除でトークンも削除。設計書に明記はないが孤児行を防ぐため）。
- 検証は Go 未導入のため shell + psql。golang-migrate は連番 SQL を順に流すだけなので同等の検証になる。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| role を PostgreSQL ENUM 型にする | 値の追加・削除に ALTER TYPE が必要でロールバックが難しい。CHECK の方が down が簡単 |
| uuid_generate_v4() | uuid-ossp 拡張が必要 |
| email に citext を使う | 拡張依存。正規化は service 層で行う方針 |

## 既存コードへの影響

なし（新規）。後続の #2（sqlc）は `schema: db/migrations` を読むため、ファイル名の連番を変えないこと。#4 は 000004 以降を使う。

## TODO・未解決事項

- [ ] CI 上で PostgreSQL 15 + 本スクリプトが通ることを確認する（push 後）
- [ ] golang-migrate 導入後に `migrate up/down` でも確認する
- [ ] チェックサム検証（IMP/Steering）は golang-migrate にチェックサム機能がないため、方式（ファイルハッシュの固定等）を #3 以降で検討
