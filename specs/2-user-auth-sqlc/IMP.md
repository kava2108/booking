---
kairo:
  node_id: "imp:2-user-auth-sqlc"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "2-user-auth-sqlc"
  feature: "user-auth"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T16:13:39+09:00"
  updated_at: "2026-10-10T16:13:39+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:2-user-auth-sqlc"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/sqlc.yaml', 'backend/db/query/user.sql', 'backend/db/query/token.sql', 'backend/internal/repository/db/*']
  band: "Green"
  last_validated: "2026-10-10T16:13:39+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 2
---

# IMP: sqlc 設定と SQL クエリファイルの作成

| 項目 | 値 |
|---|---|
| imp_id | IMP-2-user-auth-sqlc |
| imp_version | 1.0.0 |
| source_issue | #2（https://github.com/kava2108/booking/issues/2） |
| task | user-auth tasks.md §1.2 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> sqlc.yaml と users / トークン系の SQL を作成し、型安全な Querier を生成する。
> 出力先は backend/internal/repository/db/（直接使用禁止、repository interface の背後に隠す）。
> #1 のスキーマを入力とし、user-auth の P1（Domain+Repository）の前提となる。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 DB アクセス層が未整備で、repository 実装に着手できない。

### 1.2 目標
`sqlc generate` が差分なく通り、Querier と全 CRUD メソッドが生成される。

### 1.3 非目標（Out of Scope）
repository interface/実装（P1 2.1）、clinic-reservation のクエリ（#5）。

---

## 2. スニペット（変更対象の既存コード）

既存コードなし。スキーマは #1 の 000001〜000003 を参照。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/sqlc.yaml | 追加 | engine=postgresql, sql_package=pgx/v5, schema=db/migrations, queries=db/query, out=internal/repository/db | 全体 |
| backend/db/query/user.sql | 追加 | FindUserByEmail / FindUserByID / CreateUser / UpdateUserPassword | 2.1 |
| backend/db/query/token.sql | 追加 | SaveRefreshToken / FindRefreshToken / RevokeRefreshToken / SavePasswordResetToken / FindPasswordResetToken / MarkPasswordResetTokenUsed | 2.1 |
| backend/internal/repository/db/* | 追加（生成物） | sqlc generate の出力。手編集禁止 | 2.1 |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし（#1 のスキーマを入力とする） | 不要 |

---

## 4. I/O 定義

### CreateUser

**入力**
```
入力: CreateUserParams{Email string, PasswordHash string, Name string}
  例: {"a@example.com","$2a$12$…","山田"}
```

**出力**
```
出力: User{ID uuid, Role "patient", …}（role は DB の DEFAULT）
```

**エラー**
email 重複 → pgconn.PgError 23505

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| sqlc | 最新安定 | コード生成 |
| pgx/v5 | v5 | ドライバ |
| google/uuid | 最新安定 | UUID 型 |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |

---

## 6. タスク詳細

### TASK-0001: sqlc 設定と SQL クエリファイルの作成

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-001-AC-1: WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する（Must）
- [WHEN/IF] REQ-002-AC-1: WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す（Must）
- [WHEN/IF] REQ-003-AC-1: WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する（Must）
- [WHEN/IF] REQ-004-AC-1: WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する（Must）
- [WHEN/IF] REQ-005-AC-1: WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する（Must）

**実装手順**
1. sqlc.yaml を作成する（emit_interface: true）
2. user.sql / token.sql を命名規約どおりに書く（読取・書込を分ける）
3. `sqlc generate` を実行し、生成物をコミット対象にするか CI 生成にするか決める
4. CI に `sqlc generate && git diff --exit-code` を追加する

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | sqlc generate が成功し Querier が生成される |
| 異常系 | 存在しない列を参照する SQL は generate で失敗する |
| 境界値 | FindUserByEmail は小文字正規化済み入力が前提 |

**受け入れ基準チェックリスト**
- [ ] 正常系: sqlc generate がエラーなく通る
- [ ] 正常系: Querier に全メソッドが含まれる
- [ ] 異常系: スキーマ不整合の SQL は generate で検出される
- [ ] セキュリティ: クエリはすべてパラメータバインド

**依存関係**
- 前提: #1
- 後続: #3, user-auth P1 2.1, #5

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
| Unit | —（生成物） | — |
| Integration | 生成クエリを一時 DB に対して実行（#3 以降の repository テストで実施） | go test + testcontainers 等 |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: sqlc generate がエラーなく通る
- [ ] 正常系: Querier に全メソッドが含まれる
- [ ] 異常系: スキーマ不整合の SQL は generate で検出される
- [ ] セキュリティ: クエリはすべてパラメータバインド

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
生成物が壊れて後続がビルドできない場合。

### 10.2 ロールバック手順
sqlc.yaml / query を前のコミットに戻し再生成。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| 生成物のコミット方針が曖昧で差分が混入 | M | M | CI で generate の差分チェック |
| #5 とクエリファイルが衝突 | M | L | ファイル名を分離（user.sql/token.sql vs clinic 側は別名） |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] sqlc の出力パッケージが repository/db で、ハンドラー・サービスから直接参照されない構成か

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
