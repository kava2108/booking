# Requirements Document: user-auth

**ステータス**: approved
**フィーチャー**: ユーザー認証機能（登録・ログイン・JWT 発行・パスワードリセット）
**作成日**: 2026-04-11

---

## Introduction

施設予約管理システム（booking）における認証基盤フィーチャー。
一般消費者がサービスを利用するためのユーザー登録・ログイン・JWT トークン管理・パスワードリセット・ログアウトを提供する。
このフィーチャーは施設検索・予約・マイページなど全ての認証要求機能の前提となる。

---

## Requirements

### Requirement 1: ユーザー登録

**Objective**: As a 一般消費者, I want to メールアドレスとパスワードで新規登録できる, so that 施設の予約サービスを利用できる.

#### Acceptance Criteria

| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-001-AC-1 | WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する | Must |
| REQ-001-AC-2 | WHEN 同一メールアドレスが既に登録済みの場合 THEN システムは HTTP 409 とエラーメッセージを返す | Must |
| REQ-001-AC-3 | WHEN パスワードが12文字未満または必須フィールドが欠けている場合 THEN システムは HTTP 422 とフィールド単位のバリデーションエラーを返す | Must |
| REQ-001-AC-4 | WHEN 登録が完了した THEN システムは登録確認メールを送信する | Should |

---

### Requirement 2: ログイン・JWT 発行

**Objective**: As a 登録済みユーザー, I want to ログインして JWT トークンを取得できる, so that 認証が必要な機能（予約・マイページ）を利用できる.

#### Acceptance Criteria

| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-002-AC-1 | WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す | Must |
| REQ-002-AC-2 | WHEN メールアドレスまたはパスワードが誤っている場合 THEN システムは HTTP 401 を返し、どちらが誤りかは明示しない（ユーザー存在の推測防止） | Must |
| REQ-002-AC-3 | WHEN 同一 IP アドレスから5回連続でログイン失敗する THEN システムは15分間そのアドレスへの認証エンドポイントへのアクセスをレートリミットする | Must |
| REQ-002-AC-4 | WHEN ログインが成功する THEN アクセストークンは Authorization ヘッダー（Bearer）で、リフレッシュトークンは HttpOnly Cookie で返す | Must |

---

### Requirement 3: トークンリフレッシュ

**Objective**: As a ログイン中のユーザー, I want to リフレッシュトークンでアクセストークンを更新できる, so that セッションを維持しながら安全に利用できる.

#### Acceptance Criteria

| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-003-AC-1 | WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する | Must |
| REQ-003-AC-2 | WHEN リフレッシュトークンが期限切れまたは無効な場合 THEN システムは HTTP 401 を返し、再ログインを要求する | Must |
| REQ-003-AC-3 | WHEN リフレッシュトークンが使用された THEN そのトークンを無効化し（ローテーション）、新しいリフレッシュトークンを発行する | Must |

---

### Requirement 4: パスワードリセット

**Objective**: As a パスワードを忘れたユーザー, I want to メールアドレスでパスワードリセットを申請できる, so that アカウントへのアクセスを回復できる.

#### Acceptance Criteria

| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-004-AC-1 | WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する | Must |
| REQ-004-AC-2 | WHEN 未登録のメールアドレスでリセット申請があった場合 THEN システムは登録済みと同一のレスポンスを返す（メールアドレス存在の推測防止） | Must |
| REQ-004-AC-3 | WHEN ユーザーが有効なリセットトークンと新しいパスワードを送信する THEN システムはパスワードを更新し、リセットトークンを無効化する | Must |
| REQ-004-AC-4 | WHEN リセットトークンが期限切れまたは使用済みの場合 THEN システムは HTTP 400 を返す | Must |

---

### Requirement 5: ログアウト

**Objective**: As a ログイン中のユーザー, I want to ログアウトできる, so that 共有端末でも安全に利用できる.

#### Acceptance Criteria

| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-005-AC-1 | WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する | Must |
| REQ-005-AC-2 | WHEN ログアウト後にリフレッシュトークンを使用しようとした場合 THEN システムは HTTP 401 を返す | Must |

---

## Non-Functional Requirements

| カテゴリ | 要件 | 根拠（Steering） |
|---------|------|----------------|
| パフォーマンス | ログイン API は p99 で 500ms 以内に応答する | tech.md: 予約確定 3 秒以内に準拠 |
| セキュリティ | パスワードは bcrypt（cost ≥ 12）でハッシュ化する | tech.md: セキュリティ制約 |
| セキュリティ | レートリミットを認証エンドポイントに適用する（`ulule/limiter`） | tech.md: セキュリティ制約 |
| セキュリティ | JWT の秘密鍵は環境変数で管理し、ソースコードには含めない | tech.md: 秘匿情報管理 |
| セキュリティ | リフレッシュトークンは HttpOnly Cookie で管理し XSS によるトークン窃取を防ぐ | tech.md: 認証設計 |
| 可用性 | トークンリフレッシュは認証サービス単体で完結し、外部依存を最小化する | product.md: 技術制約 |
| 法的 | ユーザー個人情報（氏名・メールアドレス）は個人情報保護法に準拠して管理する | product.md: 法的制約 |

---

## Traceability

| REQ-ID | ユーザーストーリー | 設計参照 | テスト参照 |
|--------|--------------------|---------|----------|
| REQ-001 | ユーザー登録 | design.md#api-design (POST /api/v1/auth/register) | （testcases.md 生成後に更新） |
| REQ-002 | ログイン・JWT 発行 | design.md#api-design (POST /api/v1/auth/login) | （testcases.md 生成後に更新） |
| REQ-003 | トークンリフレッシュ | design.md#api-design (POST /api/v1/auth/refresh) | （testcases.md 生成後に更新） |
| REQ-004 | パスワードリセット | design.md#api-design (POST /api/v1/auth/password-reset/*) | （testcases.md 生成後に更新） |
| REQ-005 | ログアウト | design.md#api-design (POST /api/v1/auth/logout) | （testcases.md 生成後に更新） |
