# Issue 構造定義: 10-user-auth-auth-handler

## 概要
AuthHandler — 全エンドポイント実装

## フィーチャー参照
- フィーチャー: user-auth
- タスク参照: tasks.md#4.1
- 要件参照: .kiro/specs/user-auth/requirements.md
- 設計参照: .kiro/specs/user-auth/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-001-AC-1 | WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する | Must |
| REQ-001-AC-2 | WHEN 同一メールアドレスが既に登録済みの場合 THEN システムは HTTP 409 とエラーメッセージを返す | Must |
| REQ-001-AC-3 | WHEN パスワードが12文字未満または必須フィールドが欠けている場合 THEN システムは HTTP 422 とフィールド単位のバリデーションエラーを返す | Must |
| REQ-002-AC-1 | WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す | Must |
| REQ-002-AC-2 | WHEN メールアドレスまたはパスワードが誤っている場合 THEN システムは HTTP 401 を返し、どちらが誤りかは明示しない（ユーザー存在の推測防止） | Must |
| REQ-002-AC-4 | WHEN ログインが成功する THEN アクセストークンは Authorization ヘッダー（Bearer）で、リフレッシュトークンは HttpOnly Cookie で返す | Must |
| REQ-003-AC-1 | WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する | Must |
| REQ-003-AC-2 | WHEN リフレッシュトークンが期限切れまたは無効な場合 THEN システムは HTTP 401 を返し、再ログインを要求する | Must |
| REQ-004-AC-1 | WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する | Must |
| REQ-004-AC-4 | WHEN リセットトークンが期限切れまたは使用済みの場合 THEN システムは HTTP 400 を返す | Must |
| REQ-005-AC-1 | WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する | Must |

## タスク一覧
- `backend/internal/handler/auth_handler.go` を実装する
- POST /api/v1/auth/register: リクエストバリデーション（go-playground/validator）→ AuthService.Register → 201 レスポンス
- POST /api/v1/auth/login: バリデーション → AuthService.Login → アクセストークンをレスポンスボディ、リフレッシュトークンを HttpOnly Cookie（SameSite=Strict, Secure, Path=/api/v1/auth/refresh）に設定
- POST /api/v1/auth/refresh: Cookie からリフレッシュトークン取得 → AuthService.RefreshToken → トークンペア返却
- POST /api/v1/auth/logout: AuthService.Logout → Cookie クリア → 200
- POST /api/v1/auth/password-reset/request: AuthService.RequestPasswordReset → 200（常に同一レスポンス）
- POST /api/v1/auth/password-reset/confirm: AuthService.ConfirmPasswordReset → 200
- 統一エラーレスポンス形式（error, message, request_id）を実装する
- httptest を使った統合テストを作成する（全エンドポイント）

## 依存関係
- 先行タスク: user-auth 3.1, 3.2（#3 の基盤・#6 の認可ミドルウェアを利用）
- 並列波形: P1

## GitHub Issue
- Issue 番号: #10
- Issue URL: https://github.com/kava2108/booking/issues/10
