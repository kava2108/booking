# Issue 構造定義: 7-user-auth-domain-repository

## 概要
ドメインモデルと Repository interface 定義

## フィーチャー参照
- フィーチャー: user-auth
- タスク参照: tasks.md#2.1
- 要件参照: .kiro/specs/user-auth/requirements.md
- 設計参照: .kiro/specs/user-auth/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-001-AC-1 | WHEN ユーザーが有効なメールアドレス・パスワード（12文字以上）・氏名を送信する THEN システムは重複チェックを行い、パスワードを bcrypt でハッシュ化してユーザーを登録する | Must |
| REQ-002-AC-1 | WHEN ユーザーが正しいメールアドレスとパスワードを送信する THEN システムはアクセストークン（有効期限 15 分）とリフレッシュトークン（有効期限 7 日）を返す | Must |
| REQ-003-AC-1 | WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する | Must |
| REQ-004-AC-1 | WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する | Must |
| REQ-005-AC-1 | WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する | Must |

## タスク一覧
- `backend/internal/domain/user.go`: User struct（ID, Email, PasswordHash, Name, EmailVerified, Role, CreatedAt, UpdatedAt）を定義する。Role 型と定数（RolePatient / RoleStaff / RoleClinicAdmin / RoleSystemAdmin）は clinic-reservation #6 で `backend/internal/domain/role.go` に定義済みのため再定義しない
- `backend/internal/repository/user_repository.go`: UserRepository interface（Create, FindByEmail, FindByID, UpdatePassword）を定義する
- `backend/internal/repository/token_repository.go`: TokenRepository interface（SaveRefreshToken, FindRefreshToken, RevokeRefreshToken, SavePasswordResetToken, FindPasswordResetToken, MarkPasswordResetTokenUsed）を定義する
- sqlc 生成コードを使った各 interface の実装 struct を作成する

## 依存関係
- 先行タスク: #1, #2
- 並列波形: P1

## GitHub Issue
- Issue 番号: #7
- Issue URL: https://github.com/kava2108/booking/issues/7
