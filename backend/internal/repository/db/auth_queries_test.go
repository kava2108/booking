package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// IMP 2-user-auth-sqlc TASK-0001: 生成クエリを一時 DB に対して実行する統合テスト。
// TEST_DATABASE_URL（DB 名は *_test）が無ければスキップする。
func newQueries(t *testing.T) (*db.Queries, context.Context) {
	t.Helper()
	return db.New(pgtest.NewPool(t)), context.Background()
}

func newUser(t *testing.T, q *db.Queries, ctx context.Context, email string) db.User {
	t.Helper()
	u, err := q.CreateUser(ctx, db.CreateUserParams{Email: email, PasswordHash: "$2a$12$hash", Name: "山田 太郎"})
	require.NoError(t, err)
	return u
}

// REQ-001-AC-1: 登録（role は DB の既定値 patient）
func TestCreateAndGetUser(t *testing.T) {
	q, ctx := newQueries(t)

	u := newUser(t, q, ctx, "a@example.com")
	assert.NotEmpty(t, u.ID)
	assert.Equal(t, "patient", u.Role)
	assert.False(t, u.EmailVerified)

	byEmail, err := q.GetUserByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, byEmail.ID)

	byID, err := q.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "a@example.com", byID.Email)
}

// REQ-001-AC-2: 重複は一意制約違反 (23505)
func TestCreateUser_DuplicateEmail(t *testing.T) {
	q, ctx := newQueries(t)
	newUser(t, q, ctx, "dup@example.com")

	_, err := q.CreateUser(ctx, db.CreateUserParams{Email: "dup@example.com", PasswordHash: "h", Name: "n"})
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23505", pgErr.Code)
}

func TestGetUser_NotFound(t *testing.T) {
	q, ctx := newQueries(t)

	_, err := q.GetUserByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// REQ-004: パスワード更新で password_hash と updated_at が変わる
func TestUpdateUserPassword(t *testing.T) {
	q, ctx := newQueries(t)
	u := newUser(t, q, ctx, "pw@example.com")

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: u.ID, PasswordHash: "new-hash"}))

	got, err := q.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-hash", got.PasswordHash)
	assert.True(t, got.UpdatedAt.Time.After(u.UpdatedAt.Time))
}

// REQ-003: リフレッシュトークンの保存・取得・失効
func TestRefreshTokenLifecycle(t *testing.T) {
	q, ctx := newQueries(t)
	u := newUser(t, q, ctx, "rt@example.com")

	tok, err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID: u.ID, TokenHash: "hash-1", ExpiresAt: tsOf(time.Now().Add(7 * 24 * time.Hour)),
	})
	require.NoError(t, err)
	assert.False(t, tok.RevokedAt.Valid)

	got, err := q.GetRefreshTokenByHash(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, tok.ID, got.ID)

	n, err := q.RevokeRefreshToken(ctx, tok.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	revoked, err := q.GetRefreshTokenByHash(ctx, "hash-1")
	require.NoError(t, err)
	assert.True(t, revoked.RevokedAt.Valid, "失効済みでも行は返し、判定は service が行う")
}

func TestRefreshToken_UnknownHash(t *testing.T) {
	q, ctx := newQueries(t)

	_, err := q.GetRefreshTokenByHash(ctx, "missing")
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// REQ-004-AC-1: パスワードリセットトークンの保存・取得・使用済み化
func TestPasswordResetTokenLifecycle(t *testing.T) {
	q, ctx := newQueries(t)
	u := newUser(t, q, ctx, "reset@example.com")

	tok, err := q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		UserID: u.ID, TokenHash: "reset-1", ExpiresAt: tsOf(time.Now().Add(time.Hour)),
	})
	require.NoError(t, err)
	assert.False(t, tok.UsedAt.Valid)

	got, err := q.GetPasswordResetTokenByHash(ctx, "reset-1")
	require.NoError(t, err)
	assert.Equal(t, tok.ID, got.ID)

	n, err := q.MarkPasswordResetTokenUsed(ctx, tok.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	used, err := q.GetPasswordResetTokenByHash(ctx, "reset-1")
	require.NoError(t, err)
	assert.True(t, used.UsedAt.Valid)
}

// セキュリティ: 同一トークンを二重に使用済みにしても使用時刻は最初のまま（リプレイ検知に使える）
func TestMarkPasswordResetTokenUsed_Idempotent(t *testing.T) {
	q, ctx := newQueries(t)
	u := newUser(t, q, ctx, "idem@example.com")
	tok, err := q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		UserID: u.ID, TokenHash: "reset-2", ExpiresAt: tsOf(time.Now().Add(time.Hour)),
	})
	require.NoError(t, err)

	_, err = q.MarkPasswordResetTokenUsed(ctx, tok.ID)
	require.NoError(t, err)
	first, _ := q.GetPasswordResetTokenByHash(ctx, "reset-2")
	time.Sleep(10 * time.Millisecond)
	n2, err := q.MarkPasswordResetTokenUsed(ctx, tok.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n2, "2 回目は更新されない")
	second, _ := q.GetPasswordResetTokenByHash(ctx, "reset-2")

	assert.Equal(t, first.UsedAt.Time, second.UsedAt.Time)
}

// 生成された Querier が全クエリを公開している（モック生成・DI の前提）
func TestQuerierInterfaceComplete(t *testing.T) {
	var _ db.Querier = (*db.Queries)(nil)
}
