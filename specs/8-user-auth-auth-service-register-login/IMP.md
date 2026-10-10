---
kairo:
  node_id: "imp:8-user-auth-auth-service-register-login"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "8-user-auth-auth-service-register-login"
  feature: "user-auth"
  imp_version: "1.1.0"
  status: "active"
  created_at: "2026-10-10T17:01:04+09:00"
  updated_at: "2026-10-10T17:01:04+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:8-user-auth-auth-service-register-login"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/auth/token.go', 'backend/internal/service/auth_service.go', 'backend/internal/service/mailer.go', 'backend/internal/service/auth_service_test.go,', 'backend/go.mod,']
  band: "Green"
  last_validated: "2026-10-10T17:01:04+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 8
---

# IMP: AuthService interface と Register / Login 実装

| 項目 | 値 |
|---|---|
| imp_id | IMP-8-user-auth-auth-service-register-login |
| imp_version | 1.0.0 |
| source_issue | #8（https://github.com/kava2108/booking/issues/8） |
| task | user-auth tasks.md §3.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> AuthService interface、アクセストークン(JWT)/リフレッシュトークン生成、Register と Login を実装する。
> bcrypt cost 12、ユーザー存在を推測させない Login、トークンは SHA-256 ハッシュのみ保存。
> メール送信は Mailer interface 越しに非同期で行う。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 認証の中核ロジックが無い。🔵 JWT の発行・検証を service とミドルウェアで共有する必要がある（internal/auth に集約）。

### 1.2 目標
登録・ログインが mock repository でユニットテストでき、タイミング差でユーザーの存在を推測できない。

### 1.3 非目標（Out of Scope）
refresh/logout/reset（#9）、HTTP（#10）、SMTP 本実装の高度化（#14）。

---

## 2. スニペット（変更対象の既存コード）

#7 の UserRepository / TokenRepository と domain 型。config に JWT_SECRET / JWT_ACCESS_TTL / JWT_REFRESH_TTL。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/auth/token.go | 追加 | Claims、TokenManager（IssueAccess / ParseAccess）、opaque トークン生成と SHA-256 ハッシュ | #9, #10 |
| backend/internal/service/auth_service.go | 追加 | AuthService interface、Register / Login、エラー型、Mailer interface | #9, #10 |
| backend/internal/service/mailer.go | 追加 | Mailer interface、SMTPMailer、NoopMailer | #9, #10 |
| backend/internal/service/auth_service_test.go, internal/auth/token_test.go, mailer_test.go | 追加 | ユニットテスト | — |
| backend/go.mod, go.sum | 修正 | golang-jwt/v5, golang.org/x/crypto, google/uuid | — |

### 3.2 API 変更

なし（#10 で公開）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### Register

**入力**
```
入力: RegisterInput{Email " User@Example.com ", Password 12文字以上, Name}
```

**出力**
```
出力: (*domain.User, error)。email は trim + 小文字化して保存
```

**エラー**
重複 → ErrEmailAlreadyExists / パスワード 12 文字未満 → ErrWeakPassword

### Login

**入力**
```
入力: email, password
```

**出力**
```
出力: TokenPair{AccessToken, RefreshToken(raw), ExpiresIn 900, RefreshExpiresAt}
```

**エラー**
不一致・未登録とも ErrInvalidCredentials（未登録でもダミー bcrypt を実行）

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| golang-jwt/jwt | v5 | JWT (HS256) |
| golang.org/x/crypto/bcrypt | 最新安定 | パスワードハッシュ（cost 12） |
| google/uuid | 既存 | ID |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| interface{} / any の無制限使用 | 型安全性 | 具体型・ジェネリクス |
| panic の業務ロジック使用 | Steering 禁止 | error を返す |
| JWT alg=none / 署名アルゴリズムの未固定 | algorithm confusion | HS256 のみ受理 |
| 生トークンの DB 保存・ログ出力 | 漏えい | SHA-256 ハッシュのみ保存 |

---

## 6. タスク詳細

### TASK-0001: AuthService interface と Register / Login 実装

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-001-AC-1: WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する（Must）
- [WHEN/IF] REQ-001-AC-2: WHEN 同一メールアドレスが既に登録済みの場合 THEN システムは HTTP 409 とエラーメッセージを返す（Must）
- [WHEN/IF] REQ-001-AC-3: WHEN パスワードが12文字未満または必須フィールドが欠けている場合 THEN システムは HTTP 422 とフィールド単位のバリデーションエラーを返す（Must）
- [WHEN/IF] REQ-002-AC-1: WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す（Must）
- [WHEN/IF] REQ-002-AC-2: WHEN メールアドレスまたはパスワードが誤っている場合 THEN システムは HTTP 401 を返し、どちらが誤りかは明示しない（ユーザー存在の推測防止）（Must）
- [WHEN/IF] REQ-002-AC-4: WHEN ログインが成功する THEN アクセストークンは Authorization ヘッダー（Bearer）で、リフレッシュトークンは HttpOnly Cookie で返す（Must）

**実装手順**
1. internal/auth/token.go を TDD で作る（期限・改ざん・alg 固定）
2. Mailer interface と SMTPMailer/NoopMailer を作る（ヘッダーインジェクション対策）
3. AuthService interface とエラー型を定義する
4. Register: 正規化→長さ検証→bcrypt→Create→非同期メール
5. Login: FindByEmail→（未登録時ダミー比較）→bcrypt 比較→トークン発行→ハッシュ保存

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 登録・ログインでトークンペアが返る |
| 異常系 | 重複 email・弱いパスワード・誤パスワード・未登録 |
| 境界値 | パスワード 11/12 文字、email の大文字・空白 |

**受け入れ基準チェックリスト**
- [ ] 正常系: Register が bcrypt(cost12) で保存し確認メールを送る
- [ ] 異常系: 重複 → ErrEmailAlreadyExists
- [ ] 境界値: 11 文字は ErrWeakPassword、12 文字は成功
- [ ] 正常系: Login が 15 分のアクセストークンとハッシュ保存済みのリフレッシュトークンを返す
- [ ] 異常系: 誤パスワードと未登録は同じ ErrInvalidCredentials
- [ ] セキュリティ: 未登録でも bcrypt 比較を 1 回実行する
- [ ] セキュリティ: alg=none / 別鍵 / 期限切れの JWT を拒否

**依存関係**
- 前提: #7
- 後続: #9, #10

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
| Unit | 90% | testify + mockery |
| Integration | #10 で実施 | — |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: Register が bcrypt(cost12) で保存し確認メールを送る
- [ ] 異常系: 重複 → ErrEmailAlreadyExists
- [ ] 境界値: 11 文字は ErrWeakPassword、12 文字は成功
- [ ] 正常系: Login が 15 分のアクセストークンとハッシュ保存済みのリフレッシュトークンを返す
- [ ] 異常系: 誤パスワードと未登録は同じ ErrInvalidCredentials
- [ ] セキュリティ: 未登録でも bcrypt 比較を 1 回実行する
- [ ] セキュリティ: alg=none / 別鍵 / 期限切れの JWT を拒否

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
認証が不正に通る不具合が見つかった場合。

### 10.2 ロールバック手順
AuthService を DI から外してルートを無効化。DB 変更なし。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| bcrypt cost 12 でテストが遅い | L | H | サービスに cost を注入し、テストは最小 cost |
| Register の goroutine がテストで不安定 | M | M | 非同期実行関数を注入（テストは同期） |
| メール送信失敗が登録失敗にならない設計の可観測性不足 | M | M | 失敗は error ログ（request_id 付き、宛先は出さない） |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] JWT 処理が internal/auth に集約され、service と middleware が同じ実装を使っているか

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
| 1.1.0 | REQ-002-AC-3 をアカウント単位のロックに変更（AttemptGuard 追加、IP 単位は緩い天井へ）。API パスは /api/v1 に統一 | Suetsugu Kawamura | 2026-10-10 |
