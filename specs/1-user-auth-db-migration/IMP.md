---
kairo:
  node_id: "imp:1-user-auth-db-migration"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "1-user-auth-db-migration"
  feature: "user-auth"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T16:13:39+09:00"
  updated_at: "2026-10-10T16:13:39+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:1-user-auth-db-migration"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/db/migrations/000001_create_users.up.sql', 'backend/db/migrations/000001_create_users.down.sql', 'backend/db/migrations/000002_create_refresh_tokens.{up,down}.sql', 'backend/db/migrations/000003_create_password_reset_tokens.{up,down}.sql', '.github/workflows/ci.yml（該当ジョブ）']
  band: "Green"
  last_validated: "2026-10-10T16:13:39+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 1
---

# IMP: DB マイグレーション作成（users / refresh_tokens / password_reset_tokens）

| 項目 | 値 |
|---|---|
| imp_id | IMP-1-user-auth-db-migration |
| imp_version | 1.0.0 |
| source_issue | #1（https://github.com/kava2108/booking/issues/1） |
| task | user-auth tasks.md §1.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> users / refresh_tokens / password_reset_tokens を golang-migrate の up/down ペアで作成する。
> users.role は patient / staff / clinic_admin / system_admin の CHECK 制約付き（DEFAULT patient）で、clinic-reservation と共有する。
> 後続の sqlc（#2）・Go 基盤（#3）・clinic-reservation の全マイグレーションの前提となる。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 認証機能のテーブルが存在せず、sqlc 生成・API 実装に着手できない。🔵 clinic-reservation は users.role の値に依存するため、ここで先に確定する必要がある。

### 1.2 目標
`migrate up` が冪等に通り、`migrate down` で完全に戻る。CI で up/down とチェックサム検証が通る。

### 1.3 非目標（Out of Scope）
sqlc クエリ（#2）、アプリコード（#3）、clinic-reservation のテーブル（#4）。

---

## 2. スニペット（変更対象の既存コード）

既存コードなし（新規作成）。リポジトリにはコミットがなく、`backend/` は未作成。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/db/migrations/000001_create_users.up.sql | 追加 | users（uuid PK, email UK 小文字正規化, password_hash varchar(72), name, email_verified, role varchar(20) NOT NULL DEFAULT patient + CHECK, created_at, updated_at） | #2, #4 |
| backend/db/migrations/000001_create_users.down.sql | 追加 | DROP TABLE users | — |
| backend/db/migrations/000002_create_refresh_tokens.{up,down}.sql | 追加 | refresh_tokens（token_hash UK, user_id FK, expires_at, revoked_at）。IDX(token_hash), IDX(user_id, revoked_at) | #2 |
| backend/db/migrations/000003_create_password_reset_tokens.{up,down}.sql | 追加 | password_reset_tokens（token_hash UK, user_id FK, expires_at, used_at） | #2 |
| .github/workflows/ci.yml（該当ジョブ） | 追加/修正 | migrate up→down→up とチェックサム検証 | CI |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| users | 新規。role に CHECK (patient/staff/clinic_admin/system_admin) | 必要 |
| refresh_tokens | 新規 | 必要 |
| password_reset_tokens | 新規 | 必要 |

---

## 4. I/O 定義

### マイグレーション 000001〜000003

**入力**
```
入力: 空の PostgreSQL 15+ DB
  例: postgres://app@localhost/booking_test
```

**出力**
```
出力: 3 テーブル + インデックス。`schema_migrations.version = 3`
```

**エラー**
role に範囲外の値を INSERT → CHECK 違反（23514）。email 重複 → UNIQUE 違反（23505）

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| golang-migrate | v4 | up/down 実行 |
| PostgreSQL | 15+ | RDBMS |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| down.sql の省略 | up/down ペア必須（Steering） | 全 up に対応する down を作成 |
| uuid 生成の拡張依存の未検証 | uuid_generate_v4() は拡張が必要 | gen_random_uuid()（pgcrypto 不要・PG13+ 標準）を使用 |

---

## 6. タスク詳細

### TASK-0001: DB マイグレーション作成（users / refresh_tokens / password_reset_tokens）

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-001-AC-1: WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する（Must）
- [WHEN/IF] REQ-003-AC-3: WHEN リフレッシュトークンが使用された THEN そのトークンを無効化し（ローテーション）、新しいリフレッシュトークンを発行する（Must）
- [WHEN/IF] REQ-004-AC-1: WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する（Must）

**実装手順**
1. `backend/db/migrations/` を作成し 000001〜000003 の up/down を書く
2. users.role に NOT NULL DEFAULT patient と CHECK を付与する
3. 設計書 §Database Design のインデックスを付与する
4. ローカルで up→down→up を実行して冪等性を確認する
5. CI に migrate 検証ジョブを追加する

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 空 DB に up で 3 テーブルが作成される |
| 異常系 | role='user' の INSERT が CHECK 違反になる |
| 境界値 | email に大文字を含む値は service 層で小文字化する前提。DB 側は UNIQUE のみ |

**受け入れ基準チェックリスト**
- [ ] 正常系: up で 3 テーブルとインデックスが作成される
- [ ] 正常系: down で全テーブルが消え、再度 up できる
- [ ] 異常系: role に不正値を INSERT すると CHECK 違反
- [ ] 異常系: 同一 email の二重 INSERT が UNIQUE 違反
- [ ] セキュリティ: password_hash は varchar(72) で平文用の列がない

**依存関係**
- 前提: なし（P0 先頭）
- 後続: #2, #3, #4, #5, #6

---

## 7. 禁止事項

- [ ] 本 Issue の「3.1 変更ファイル一覧」以外のファイルを変更しないこと
- [ ] 他 Issue が所有するファイル・マイグレーション番号を変更しないこと
- [ ] 既存のテストを削除・修正しないこと
- [ ] 新たな外部依存を追加しないこと（5.1 に無いもの。必要なら IMP を更新）
- [ ] 禁止パターン（5.2）を使わないこと

---

## 8. テスト戦略

### 8.1 テスト方針

| レベル | カバレッジ目標 | 使用フレームワーク |
|---|---|---|
| Unit | —（SQL のため対象外） | — |
| Integration | up/down/up の全パターン | golang-migrate + 一時 PostgreSQL |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: up で 3 テーブルとインデックスが作成される
- [ ] 正常系: down で全テーブルが消え、再度 up できる
- [ ] 異常系: role に不正値を INSERT すると CHECK 違反
- [ ] 異常系: 同一 email の二重 INSERT が UNIQUE 違反
- [ ] セキュリティ: password_hash は varchar(72) で平文用の列がない

### 8.3 テスト除外事項
生成物（sqlc 出力・mockery 出力）は直接テストしない。

### 8.4 テストデータ方針
個人情報を含む実データは使用しない。テスト用の固定 UUID・ダミー氏名のみ。

---

## 9. Definition of Done

- [ ] 全テストケースが通過している
- [ ] 型エラー・ビルドエラーが 0 件である
- [ ] 既存テストが破壊されていない
- [ ] 禁止事項（セクション 7）を全て遵守している
- [ ] IMP の受け入れ基準（EARS）を全て満たしている
- [ ] コードレビュー指摘事項が解消されている

---

## 10. ロールバック計画

### 10.1 ロールバックトリガー条件
CI で up/down が失敗する、または後続 Issue で列定義の変更が必要になった場合。

### 10.2 ロールバック手順
`migrate down 3` を実行し、ファイルを修正して再度 up。
未デプロイのためデータ移行は不要。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| role の値が user-auth 元設計（user/facility_admin）と異なる | M | L | 設計書・タスクを 2026-10-10 に更新済み。#3 の Role 定数もこの値に揃える |
| uuid 生成関数の拡張依存 | L | L | gen_random_uuid() を使用 |
| 番号衝突（clinic 側 000004〜） | M | M | clinic は 000004 以降と明記済み。CI で連番ギャップを検出 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] clinic-reservation のマイグレーションが 000004 以降で連番になっているか

---

## 13. 承認記録

| ロール | 担当者 | 承認日時 | コメント |
|---|---|---|---|
| アーキテクト | | | |
| 実装者 | | | |

---

## Appendix: 変更履歴

| バージョン | 変更内容 | 変更者 | 日時 |
|---|---|---|---|
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T16:13:39+09:00 |
