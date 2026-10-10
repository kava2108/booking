package domain

import (
	"time"

	"github.com/google/uuid"
)

// User は認証対象のユーザー。PasswordHash は認証処理のために保持するが、
// HTTP レスポンスには絶対に含めない（handler の DTO で除外する）。
type User struct {
	ID            uuid.UUID
	Email         string
	PasswordHash  string
	Name          string
	EmailVerified bool
	Role          Role
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// RefreshToken は保存済みリフレッシュトークン。生トークンは保持せず SHA-256 ハッシュのみ。
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// IsUsable は失効しておらず、期限内（期限ちょうどは無効）であれば true。
func (t RefreshToken) IsUsable(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// PasswordResetToken は保存済みのパスワードリセットトークン（ハッシュのみ）。
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// IsUsable は未使用で、期限内（期限ちょうどは無効）であれば true。
func (t PasswordResetToken) IsUsable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
