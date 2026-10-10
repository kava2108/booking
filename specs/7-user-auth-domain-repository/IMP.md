---
kairo:
  node_id: "imp:7-user-auth-domain-repository"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "7-user-auth-domain-repository"
  feature: "user-auth"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T17:01:04+09:00"
  updated_at: "2026-10-10T17:01:04+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:7-user-auth-domain-repository"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/domain/user.go', 'backend/internal/repository/errors.go', 'backend/internal/repository/user_repository.go', 'backend/internal/repository/token_repository.go', 'backend/db/query/token.sql', 'backend/internal/repository/db/*（生成物）', 'backend/internal/repository/db/auth_queries_test.go', 'backend/.mockery.yaml,']
  band: "Green"
  last_validated: "2026-10-10T17:01:04+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 7
---

# IMP: ドメインモデルと Repository interface 定義

| 項目 | 値 |
|---|---|
| imp_id | IMP-7-user-auth-domain-repository |
| imp_version | 1.0.0 |
| source_issue | #7（https://github.com/kava2108/booking/issues/7） |
| task | user-auth tasks.md §2.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> User / RefreshToken / PasswordResetToken のドメイン型と、UserRepository / TokenRepository interface を定義し、sqlc の db.Querier を包む実装を作る。
> DB 固有のエラー（no rows / 一意制約違反）をドメインエラーに変換し、service 層が pgx に依存しないようにする。
> 以降の service（#8, #9）が mockery モックで単体テストできる土台。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 service が sqlc 生成型・pgx エラーに直接依存すると Steering 違反でテストも困難。🔵 トークンのリフレッシュ/リセットは競合時の二重使用を防ぐため「行を更新できたか」を返す必要がある。

### 1.2 目標
interface 経由で DI でき、ドメインエラーに変換され、モックが生成される。失効・使用済み化が原子的で、更新件数を返す。

### 1.3 非目標（Out of Scope）
service ロジック（#8, #9）、HTTP（#10）、clinic の repository（#5 で完了）。

---

## 2. スニペット（変更対象の既存コード）

backend/internal/repository/db/（sqlc 生成）。User / RefreshToken / PasswordResetToken の row 型と Querier interface が存在。RevokeRefreshToken / MarkPasswordResetTokenUsed は :exec（更新件数が取れない）。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/domain/user.go | 追加 | User / RefreshToken / PasswordResetToken と有効性判定メソッド（Role は role.go を再利用） | #8, #9, #10 |
| backend/internal/repository/errors.go | 追加 | ErrNotFound / ErrEmailTaken | #8, #9 |
| backend/internal/repository/user_repository.go | 追加 | UserRepository interface と実装（Create, FindByEmail, FindByID, UpdatePassword） | #8, #9 |
| backend/internal/repository/token_repository.go | 追加 | TokenRepository interface と実装 | #9 |
| backend/db/query/token.sql | 修正 | RevokeRefreshToken / MarkPasswordResetTokenUsed を :execrows に、RevokeAllRefreshTokensForUser を追加 | #9 |
| backend/internal/repository/db/*（生成物） | 修正 | 再生成 | — |
| backend/internal/repository/db/auth_queries_test.go | 修正 | 戻り値の変更に追随＋新クエリのテスト | — |
| backend/.mockery.yaml, internal/repository/mocks/* | 修正 | UserRepository / TokenRepository を追加 | #8, #9 |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### UserRepository.Create

**入力**
```
入力: ctx, CreateUserParams{Email "a@example.com"（小文字正規化済み）, PasswordHash, Name}
```

**出力**
```
出力: (*domain.User, error)
```

**エラー**
email 重複 → ErrEmailTaken。その他 → wrapped error

### TokenRepository.RevokeRefreshToken

**入力**
```
入力: ctx, id uuid
```

**出力**
```
出力: (revoked bool, error)。既に失効済み・存在しない場合は revoked=false
```

**エラー**
DB 障害 → wrapped error

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| sqlc / pgx/v5 | 既存 | DB アクセス |
| mockery | v2.53.5 | モック生成 |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| interface{} / any の無制限使用 | 型安全性 | 具体型・ジェネリクス |
| panic の業務ロジック使用 | Steering 禁止 | error を返す |
| service への db.* 型・pgx エラーの露出 | 層の分離 | domain 型とドメインエラーに変換 |

---

## 6. タスク詳細

### TASK-0001: ドメインモデルと Repository interface 定義

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-001-AC-1: WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する（Must）
- [WHEN/IF] REQ-002-AC-1: WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す（Must）
- [WHEN/IF] REQ-003-AC-1: WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する（Must）
- [WHEN/IF] REQ-004-AC-1: WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する（Must）
- [WHEN/IF] REQ-005-AC-1: WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する（Must）

**実装手順**
1. domain/user.go に型を定義する（トークンの IsUsable(now) 判定を含む）
2. repository/errors.go にドメインエラーを定義する
3. token.sql を :execrows 化し RevokeAllRefreshTokensForUser を追加して再生成する
4. db.Querier を受け取る実装を作り pgx.ErrNoRows / 23505 を変換する
5. mockery 設定に追加してモックを再生成する
6. 統合テストで変換・原子性を検証する

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | Create → FindByEmail / FindByID で同じ値が返る |
| 異常系 | email 重複で ErrEmailTaken、未登録で ErrNotFound |
| 境界値 | 二重の失効/使用済み化は 2 回目 false。期限ちょうどのトークンは無効 |

**受け入れ基準チェックリスト**
- [ ] 正常系: Create/Find が domain 型にマップされる（role=patient）
- [ ] 異常系: 重複 email → ErrEmailTaken
- [ ] 異常系: 未登録 → ErrNotFound
- [ ] 境界値: 二重失効の 2 回目は false
- [ ] 正常系: RevokeAll が有効なトークンだけを失効する
- [ ] セキュリティ: UserRepository の戻り値以外から password_hash が漏れない（DTO は #10）

**依存関係**
- 前提: #1, #2
- 後続: #8, #9, #10

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
| Unit | domain の有効性判定 100% | go test |
| Integration | repository の全メソッド | pgtest（専用スキーマ） |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: Create/Find が domain 型にマップされる（role=patient）
- [ ] 異常系: 重複 email → ErrEmailTaken
- [ ] 異常系: 未登録 → ErrNotFound
- [ ] 境界値: 二重失効の 2 回目は false
- [ ] 正常系: RevokeAll が有効なトークンだけを失効する
- [ ] セキュリティ: UserRepository の戻り値以外から password_hash が漏れない（DTO は #10）

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
interface 変更で #8〜#10 が大きく手戻りする場合。

### 10.2 ロールバック手順
repository 層のコミットを revert（DB スキーマ変更なし）。sqlc 変更は token.sql を戻して再生成。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| #2 のクエリ変更（:execrows）で #2 のテストが壊れる | L | H | 同一 PR でテストを更新し CI で検出 |
| domain 型と db 型の二重定義で乖離 | M | M | マッピングを 1 箇所に集約しテストで担保 |
| interface が大きくなりすぎる | L | M | user-auth 範囲のメソッドのみ。clinic 側と混ぜない |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] domain が repository/db に依存していないか（依存方向 domain ← repository）

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
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T17:01:04+09:00 |
