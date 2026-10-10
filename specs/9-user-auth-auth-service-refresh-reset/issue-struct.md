# Issue 構造定義: 9-user-auth-auth-service-refresh-reset

## 概要
AuthService — RefreshToken / Logout / PasswordReset 実装

## フィーチャー参照
- フィーチャー: user-auth
- タスク参照: tasks.md#3.2
- 要件参照: .kiro/specs/user-auth/requirements.md
- 設計参照: .kiro/specs/user-auth/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-003-AC-1 | WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する | Must |
| REQ-003-AC-2 | WHEN リフレッシュトークンが期限切れまたは無効な場合 THEN システムは HTTP 401 を返し、再ログインを要求する | Must |
| REQ-003-AC-3 | WHEN リフレッシュトークンが使用された THEN そのトークンを無効化し（ローテーション）、新しいリフレッシュトークンを発行する | Must |
| REQ-004-AC-1 | WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する | Must |
| REQ-004-AC-2 | WHEN 未登録のメールアドレスでリセット申請があった場合 THEN システムは登録済みと同一のレスポンスを返す（メールアドレス存在の推測防止） | Must |
| REQ-004-AC-3 | WHEN ユーザーが有効なリセットトークンと新しいパスワードを送信する THEN システムはパスワードを更新し、リセットトークンを無効化する | Must |
| REQ-004-AC-4 | WHEN リセットトークンが期限切れまたは使用済みの場合 THEN システムは HTTP 400 を返す | Must |
| REQ-005-AC-1 | WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する | Must |
| REQ-005-AC-2 | WHEN ログアウト後にリフレッシュトークンを使用しようとした場合 THEN システムは HTTP 401 を返す | Must |

## タスク一覧
- RefreshToken 実装: rawToken を SHA-256 ハッシュ化 → FindRefreshToken → 期限・取り消しチェック → RevokeRefreshToken → 新規トークンペア生成・保存（ローテーション）
- Logout 実装: Cookie からリフレッシュトークン取得 → RevokeRefreshToken → Cookie 削除
- RequestPasswordReset 実装: FindByEmail（未存在でも同一レスポンス） → リセットトークン生成 → token_hash 保存 → goroutine でメール送信
- ConfirmPasswordReset 実装: FindPasswordResetToken → 期限・使用済みチェック → UpdatePassword(bcrypt) → MarkPasswordResetTokenUsed
- 各メソッドのユニットテストを作成する

## 依存関係
- 先行タスク: user-auth 3.1
- 並列波形: P1

## GitHub Issue
- Issue 番号: #9
- Issue URL: https://github.com/kava2108/booking/issues/9
