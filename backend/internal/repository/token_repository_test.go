package repository_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

func newTokenRepos(t *testing.T) (repository.UserRepository, repository.TokenRepository, *domain.User) {
	t.Helper()
	q := db.New(pgtest.NewPool(t))
	users := repository.NewUserRepository(q)
	u, err := users.Create(context.Background(), repository.CreateUserParams{Email: "t@example.com", PasswordHash: "h", Name: "n"})
	require.NoError(t, err)
	return users, repository.NewTokenRepository(q), u
}

func TestRefreshToken_SaveFindRevoke(t *testing.T) {
	_, tokens, u := newTokenRepos(t)
	ctx := context.Background()
	exp := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Microsecond)

	saved, err := tokens.SaveRefreshToken(ctx, u.ID, "hash-1", exp)
	require.NoError(t, err)
	assert.Equal(t, u.ID, saved.UserID)
	assert.Nil(t, saved.RevokedAt)

	got, err := tokens.FindRefreshToken(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
	assert.True(t, got.ExpiresAt.Equal(exp))
	assert.True(t, got.IsUsable(time.Now()))

	revoked, err := tokens.RevokeRefreshToken(ctx, got.ID)
	require.NoError(t, err)
	assert.True(t, revoked)

	after, err := tokens.FindRefreshToken(ctx, "hash-1")
	require.NoError(t, err)
	assert.NotNil(t, after.RevokedAt)
	assert.False(t, after.IsUsable(time.Now()))
}

func TestRefreshToken_NotFound(t *testing.T) {
	_, tokens, _ := newTokenRepos(t)

	_, err := tokens.FindRefreshToken(context.Background(), "missing")
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

// 二重失効の 2 回目は false（service がローテーション競合を検知できる）
func TestRevokeRefreshToken_SecondCallReturnsFalse(t *testing.T) {
	_, tokens, u := newTokenRepos(t)
	ctx := context.Background()
	tok, err := tokens.SaveRefreshToken(ctx, u.ID, "h", time.Now().Add(time.Hour))
	require.NoError(t, err)

	first, err := tokens.RevokeRefreshToken(ctx, tok.ID)
	require.NoError(t, err)
	second, err := tokens.RevokeRefreshToken(ctx, tok.ID)
	require.NoError(t, err)
	assert.True(t, first)
	assert.False(t, second)

	missing, err := tokens.RevokeRefreshToken(ctx, uuid.New())
	require.NoError(t, err)
	assert.False(t, missing)
}

// 並行失効でも成功は 1 件のみ
func TestRevokeRefreshToken_ConcurrentOnlyOneWins(t *testing.T) {
	_, tokens, u := newTokenRepos(t)
	ctx := context.Background()
	tok, err := tokens.SaveRefreshToken(ctx, u.ID, "h", time.Now().Add(time.Hour))
	require.NoError(t, err)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := tokens.RevokeRefreshToken(ctx, tok.ID)
			if assert.NoError(t, err) && ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 1, wins.Load())
}

func TestRevokeAllRefreshTokens_OnlyActiveTokensOfThatUser(t *testing.T) {
	users, tokens, u := newTokenRepos(t)
	ctx := context.Background()
	other, err := users.Create(ctx, repository.CreateUserParams{Email: "o@example.com", PasswordHash: "h", Name: "o"})
	require.NoError(t, err)

	a, _ := tokens.SaveRefreshToken(ctx, u.ID, "a", time.Now().Add(time.Hour))
	_, _ = tokens.SaveRefreshToken(ctx, u.ID, "b", time.Now().Add(time.Hour))
	_, _ = tokens.SaveRefreshToken(ctx, other.ID, "c", time.Now().Add(time.Hour))
	_, err = tokens.RevokeRefreshToken(ctx, a.ID) // 既に失効済み

	n, err := tokens.RevokeAllRefreshTokens(ctx, u.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "未失効の b のみ")

	c, err := tokens.FindRefreshToken(ctx, "c")
	require.NoError(t, err)
	assert.Nil(t, c.RevokedAt, "他ユーザーのトークンは影響を受けない")
}

func TestPasswordResetToken_SaveFindMarkUsed(t *testing.T) {
	_, tokens, u := newTokenRepos(t)
	ctx := context.Background()

	saved, err := tokens.SavePasswordResetToken(ctx, u.ID, "reset-1", time.Now().Add(30*time.Minute))
	require.NoError(t, err)
	assert.Nil(t, saved.UsedAt)

	got, err := tokens.FindPasswordResetToken(ctx, "reset-1")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
	assert.True(t, got.IsUsable(time.Now()))

	first, err := tokens.MarkPasswordResetTokenUsed(ctx, got.ID)
	require.NoError(t, err)
	second, err := tokens.MarkPasswordResetTokenUsed(ctx, got.ID)
	require.NoError(t, err)
	assert.True(t, first)
	assert.False(t, second, "二重使用は検知できる")

	used, _ := tokens.FindPasswordResetToken(ctx, "reset-1")
	assert.False(t, used.IsUsable(time.Now()))
}

func TestPasswordResetToken_NotFound(t *testing.T) {
	_, tokens, _ := newTokenRepos(t)

	_, err := tokens.FindPasswordResetToken(context.Background(), "missing")
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
