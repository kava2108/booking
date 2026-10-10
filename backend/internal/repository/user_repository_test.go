package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// IMP 7-user-auth-domain-repository TASK-0001

func newUserRepo(t *testing.T) repository.UserRepository {
	t.Helper()
	return repository.NewUserRepository(db.New(pgtest.NewPool(t)))
}

func TestUserRepository_CreateAndFind(t *testing.T) {
	r, ctx := newUserRepo(t), context.Background()

	u, err := r.Create(ctx, repository.CreateUserParams{Email: "a@example.com", PasswordHash: "hash", Name: "山田 太郎"})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, u.ID)
	assert.Equal(t, domain.RolePatient, u.Role, "既定ロールは patient")
	assert.False(t, u.EmailVerified)
	assert.False(t, u.CreatedAt.IsZero())

	byEmail, err := r.FindByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, u, byEmail)

	byID, err := r.FindByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u, byID)
	assert.Equal(t, "hash", byID.PasswordHash, "service が検証に使うため domain には含む（DTO で除外する）")
}

func TestUserRepository_DuplicateEmailMapsToErrEmailTaken(t *testing.T) {
	r, ctx := newUserRepo(t), context.Background()
	_, err := r.Create(ctx, repository.CreateUserParams{Email: "dup@example.com", PasswordHash: "h", Name: "n"})
	require.NoError(t, err)

	_, err = r.Create(ctx, repository.CreateUserParams{Email: "dup@example.com", PasswordHash: "h", Name: "n"})
	assert.ErrorIs(t, err, repository.ErrEmailTaken)
}

func TestUserRepository_NotFoundMapsToErrNotFound(t *testing.T) {
	r, ctx := newUserRepo(t), context.Background()

	_, err := r.FindByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, repository.ErrNotFound)
	_, err = r.FindByID(ctx, uuid.New())
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	r, ctx := newUserRepo(t), context.Background()
	u, err := r.Create(ctx, repository.CreateUserParams{Email: "pw@example.com", PasswordHash: "old", Name: "n"})
	require.NoError(t, err)

	require.NoError(t, r.UpdatePassword(ctx, u.ID, "new"))
	got, err := r.FindByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "new", got.PasswordHash)
	assert.True(t, got.UpdatedAt.After(u.UpdatedAt) || got.UpdatedAt.Equal(u.UpdatedAt))
}
