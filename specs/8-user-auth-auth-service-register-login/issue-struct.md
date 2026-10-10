# Issue 構造定義: 8-user-auth-auth-service-register-login

## 概要
AuthService interface と Register / Login 実装

## フィーチャー参照
- フィーチャー: user-auth
- タスク参照: tasks.md#3.1
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

## タスク一覧
- `backend/internal/service/auth_service.go`: AuthService interface を定義する（Register, Login, RefreshToken, Logout, RequestPasswordReset, ConfirmPasswordReset）
- Register 実装: email 重複チェック → bcrypt ハッシュ化（cost=12）→ DB 保存 → 確認メールを goroutine で非同期送信
- Login 実装: FindByEmail → bcrypt.CompareHashAndPassword → アクセストークン(15分) + リフレッシュトークン(7日)生成 → token_hash(SHA-256) を DB 保存
- エラー型（ErrEmailAlreadyExists, ErrInvalidCredentials）を定義する
- mockery で UserRepository / TokenRepository のモックを生成してユニットテストを作成する

## 依存関係
- 先行タスク: user-auth 2.1
- 並列波形: P1

## GitHub Issue
- Issue 番号: #8
- Issue URL: https://github.com/kava2108/booking/issues/8
