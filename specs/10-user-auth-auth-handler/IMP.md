---
kairo:
  node_id: "imp:10-user-auth-auth-handler"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "10-user-auth-auth-handler"
  feature: "user-auth"
  imp_version: "1.1.0"
  status: "active"
  created_at: "2026-10-10T17:01:04+09:00"
  updated_at: "2026-10-10T17:01:04+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:10-user-auth-auth-handler"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/handler/auth_handler.go', 'backend/internal/handler/errors.go', 'backend/internal/handler/validator.go', 'backend/internal/middleware/jwt_auth.go', 'backend/internal/config/config.go', 'backend/cmd/api/main.go', 'backend/internal/handler/*_test.go,']
  band: "Green"
  last_validated: "2026-10-10T17:01:04+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 10
---

# IMP: AuthHandler — 全エンドポイント実装

| 項目 | 値 |
|---|---|
| imp_id | IMP-10-user-auth-auth-handler |
| imp_version | 1.0.0 |
| source_issue | #10（https://github.com/kava2108/booking/issues/10） |
| task | user-auth tasks.md §4.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> /api/v1/auth の6エンドポイント、JWT 検証ミドルウェア（Identity を設定）、統一エラー形式、DB・サービスの結線を実装する。
> 認証系にレートリミット、リフレッシュトークンは HttpOnly + SameSite=Strict の Cookie。
> clinic-reservation #6 の認可・監査ミドルウェアが前提とする Identity の供給元になる。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 HTTP の入口が無い。🔵 jwt_auth が未実装で、clinic の StaffOnly/AdminOnly が機能しない。

### 1.2 目標
全エンドポイントが統合テスト（実 DB）で動き、登録→ログイン→更新→ログアウト→更新失敗が通る。

### 1.3 非目標（Out of Scope）
フロント（P2）、OpenAPI（5.1）、メール文面の凝った実装。

---

## 2. スニペット（変更対象の既存コード）

#3 の newEcho（request_id / logger / recover / healthz）と NewRateLimiter。#6 の SetIdentity / RequireRole。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/handler/auth_handler.go | 追加 | 6 エンドポイントのハンドラー、DTO、Cookie 処理 | — |
| backend/internal/handler/errors.go | 追加 | 統一エラー形式 {error, message, request_id, details} | 全ハンドラー |
| backend/internal/handler/validator.go | 追加 | go-playground/validator の Echo 結線とフィールドエラー変換 | 全ハンドラー |
| backend/internal/middleware/jwt_auth.go | 追加 | echo-jwt ラッパー。検証成功時に Identity を設定 | clinic API 全般 |
| backend/internal/config/config.go | 修正 | COOKIE_SECURE（既定 true）、APP_BASE_URL を追加 | — |
| backend/cmd/api/main.go | 修正 | DB 接続（pgxpool）、service・handler の結線、ルート登録 | — |
| backend/internal/handler/*_test.go, middleware/jwt_auth_test.go, cmd/api/*_test.go | 追加 | httptest + 実 DB の統合テスト | — |

### 3.2 API 変更

POST /api/v1/auth/{register,login,refresh,logout,password-reset/request,password-reset/confirm}（新規）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### POST /api/v1/auth/login

**入力**
```
入力: {"email":"a@example.com","password":"…"}
```

**出力**
```
出力: 200 {"access_token","token_type":"Bearer","expires_in":900} + Set-Cookie refresh_token
```

**エラー**
不一致 401 / 429 レートリミット / 422 バリデーション

### POST /api/v1/auth/logout

**入力**
```
入力: Authorization: Bearer + Cookie
```

**出力**
```
出力: 200、Cookie 削除
```

**エラー**
Bearer 無し 401。Cookie 無しでも 200（冪等）

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| labstack/echo-jwt | v4 | JWT 検証 |
| go-playground/validator | v10 | 入力検証 |
| ulule/limiter | v3 | 認証系レートリミット（#3） |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| interface{} / any の無制限使用 | 型安全性 | 具体型・ジェネリクス |
| panic の業務ロジック使用 | Steering 禁止 | error を返す |
| Cookie の Secure/HttpOnly/SameSite を緩める | XSS・CSRF | Strict/HttpOnly 固定。Secure のみ開発用に設定可 |
| エラー応答に内部エラー文字列を含める | 情報漏えい | 汎用メッセージ + request_id |

---

## 6. タスク詳細

### TASK-0001: AuthHandler — 全エンドポイント実装

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-001-AC-1: WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する（Must）
- [WHEN/IF] REQ-001-AC-2: WHEN 同一メールアドレスが既に登録済みの場合 THEN システムは HTTP 409 とエラーメッセージを返す（Must）
- [WHEN/IF] REQ-001-AC-3: WHEN パスワードが12文字未満または必須フィールドが欠けている場合 THEN システムは HTTP 422 とフィールド単位のバリデーションエラーを返す（Must）
- [WHEN/IF] REQ-002-AC-1: WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す（Must）
- [WHEN/IF] REQ-002-AC-2: WHEN メールアドレスまたはパスワードが誤っている場合 THEN システムは HTTP 401 を返し、どちらが誤りかは明示しない（ユーザー存在の推測防止）（Must）
- [WHEN/IF] REQ-002-AC-4: WHEN ログインが成功する THEN アクセストークンは Authorization ヘッダー（Bearer）で、リフレッシュトークンは HttpOnly Cookie で返す（Must）
- [WHEN/IF] REQ-003-AC-1: WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する（Must）
- [WHEN/IF] REQ-003-AC-2: WHEN リフレッシュトークンが期限切れまたは無効な場合 THEN システムは HTTP 401 を返し、再ログインを要求する（Must）
- [WHEN/IF] REQ-004-AC-1: WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する（Must）
- [WHEN/IF] REQ-004-AC-4: WHEN リセットトークンが期限切れまたは使用済みの場合 THEN システムは HTTP 400 を返す（Must）
- [WHEN/IF] REQ-005-AC-1: WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する（Must）

**実装手順**
1. 統一エラー形式とバリデーションを作る
2. jwt_auth ミドルウェアを TDD で作る（有効/期限切れ/改ざん/alg）
3. AuthHandler を実装しルート登録関数を作る
4. main.go で DB とサービスを結線する
5. 実 DB の統合テストで全フロー・レートリミット・Cookie 属性を確認する

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | 登録→ログイン→更新→ログアウトの一連フロー |
| 異常系 | 重複 409 / 弱いパスワード 422 / 誤認証 401 / 無効 Cookie 401 |
| 境界値 | 5 回目まで通過、6 回目で 429 |

**受け入れ基準チェックリスト**
- [ ] 正常系: 6 エンドポイントのハッピーパス
- [ ] 異常系: register 409 / 422、login 401、refresh Cookie 無し 401
- [ ] 異常系: logout 後の refresh 401
- [ ] 正常系: Set-Cookie が HttpOnly・SameSite=Strict・Path=/api/v1/auth
- [ ] 境界値: login を 6 回で 429 + Retry-After
- [ ] セキュリティ: password-reset/request は未登録でも同一応答
- [ ] セキュリティ: 応答・ログに password_hash やトークン原文が出ない
- [ ] 正常系: jwt_auth が Identity(UserID, Role) を設定し StaffOnly と連携

**依存関係**
- 前提: #8, #9（#3 の基盤、#6 のミドルウェア）
- 後続: user-auth P2、clinic P1 全ハンドラー

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
| Unit | ハンドラー 80%+ | testify + httptest |
| Integration | 全エンドポイント | pgtest + httptest |
| E2E | P2 6.1 | Playwright |

### 8.2 テスト観点

- [ ] 正常系: 6 エンドポイントのハッピーパス
- [ ] 異常系: register 409 / 422、login 401、refresh Cookie 無し 401
- [ ] 異常系: logout 後の refresh 401
- [ ] 正常系: Set-Cookie が HttpOnly・SameSite=Strict・Path=/api/v1/auth
- [ ] 境界値: login を 6 回で 429 + Retry-After
- [ ] セキュリティ: password-reset/request は未登録でも同一応答
- [ ] セキュリティ: 応答・ログに password_hash やトークン原文が出ない
- [ ] 正常系: jwt_auth が Identity(UserID, Role) を設定し StaffOnly と連携

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
認証バイパスや Cookie 設定ミスが見つかった場合。

### 10.2 ロールバック手順
/api/v1/auth ルートを無効化して前コミットへ revert。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| Cookie の Path を設計書(/refresh)のままにすると logout に Cookie が届かない | M | H | Path=/api/v1/auth とし、設計書との差異を記録 |
| レートリミットを成功ログインにも数える仕様差 | M | M | #3 の記録どおり。必要なら成功時 Reset を追加 |
| main の結線で DB 障害時に起動が曖昧 | M | M | 起動時に Ping し失敗なら終了 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] handler が service interface のみに依存し、db/pgx を直接参照していないか

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
