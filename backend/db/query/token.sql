-- IMP 2-user-auth-sqlc TASK-0001
-- token_hash は SHA-256 の 16 進文字列。生トークンは渡さない。

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
-- 失効済み・期限切れも返す。有効性の判定は service 層で行う。
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshToken :execrows
-- 影響行数 0 = 既に失効済み（または存在しない）。ローテーションの競合検知に使う。
UPDATE refresh_tokens
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPasswordResetTokenByHash :one
SELECT * FROM password_reset_tokens WHERE token_hash = $1;

-- name: MarkPasswordResetTokenUsed :execrows
-- 影響行数 0 = 既に使用済み（または存在しない）。
UPDATE password_reset_tokens
SET used_at = now()
WHERE id = $1 AND used_at IS NULL;

-- name: RevokeAllRefreshTokensForUser :execrows
-- パスワード変更時に全セッションを失効させる。
UPDATE refresh_tokens
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;
