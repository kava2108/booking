package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

// TokenRepository はリフレッシュトークンとパスワードリセットトークンの永続化。
// token_hash には SHA-256 の 16 進文字列だけを渡す（生トークンは保存しない）。
// Revoke / MarkUsed は「更新できたか」を返し、同時リクエストの二重使用を service が検知できるようにする。
// IMP 7-user-auth-domain-repository TASK-0001
type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*domain.RefreshToken, error)
	// FindRefreshToken は未登録のとき ErrNotFound。失効・期限切れでも行は返す（判定は IsUsable）。
	FindRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	// RevokeRefreshToken は新たに失効できたとき true。既に失効済み・存在しない場合は false。
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) (bool, error)
	// RevokeAllRefreshTokens はユーザーの未失効トークンをすべて失効し、件数を返す。
	RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) (int64, error)

	SavePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*domain.PasswordResetToken, error)
	FindPasswordResetToken(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error)
	// MarkPasswordResetTokenUsed は新たに使用済みにできたとき true。
	MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) (bool, error)
}

type tokenRepository struct{ q db.Querier }

func NewTokenRepository(q db.Querier) TokenRepository { return &tokenRepository{q: q} }

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func optTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func toRefresh(t db.RefreshToken) *domain.RefreshToken {
	return &domain.RefreshToken{ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash, ExpiresAt: t.ExpiresAt.Time, RevokedAt: optTime(t.RevokedAt), CreatedAt: t.CreatedAt.Time}
}

func toReset(t db.PasswordResetToken) *domain.PasswordResetToken {
	return &domain.PasswordResetToken{ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash, ExpiresAt: t.ExpiresAt.Time, UsedAt: optTime(t.UsedAt), CreatedAt: t.CreatedAt.Time}
}

func (r *tokenRepository) SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*domain.RefreshToken, error) {
	row, err := r.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{UserID: userID, TokenHash: tokenHash, ExpiresAt: ts(expiresAt)})
	if err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}
	return toRefresh(row), nil
}

func (r *tokenRepository) FindRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, mapFindErr("find refresh token", err)
	}
	return toRefresh(row), nil
}

func (r *tokenRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := r.q.RevokeRefreshToken(ctx, id)
	if err != nil {
		return false, fmt.Errorf("revoke refresh token: %w", err)
	}
	return n > 0, nil
}

func (r *tokenRepository) RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) (int64, error) {
	n, err := r.q.RevokeAllRefreshTokensForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return n, nil
}

func (r *tokenRepository) SavePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*domain.PasswordResetToken, error) {
	row, err := r.q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{UserID: userID, TokenHash: tokenHash, ExpiresAt: ts(expiresAt)})
	if err != nil {
		return nil, fmt.Errorf("save password reset token: %w", err)
	}
	return toReset(row), nil
}

func (r *tokenRepository) FindPasswordResetToken(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error) {
	row, err := r.q.GetPasswordResetTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, mapFindErr("find password reset token", err)
	}
	return toReset(row), nil
}

func (r *tokenRepository) MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := r.q.MarkPasswordResetTokenUsed(ctx, id)
	if err != nil {
		return false, fmt.Errorf("mark password reset token used: %w", err)
	}
	return n > 0, nil
}
