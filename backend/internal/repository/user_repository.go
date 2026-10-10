package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

// CreateUserParams は新規ユーザーの作成パラメーター。Email は正規化（小文字・trim）済みであること。
type CreateUserParams struct {
	Email        string
	PasswordHash string
	Name         string
}

// UserRepository はユーザーの永続化。service は pgx / sqlc 型を知らずにこれだけに依存する。
// IMP 7-user-auth-domain-repository TASK-0001
type UserRepository interface {
	// Create は email 重複のとき ErrEmailTaken を返す。
	Create(ctx context.Context, p CreateUserParams) (*domain.User, error)
	// FindByEmail / FindByID は未登録のとき ErrNotFound を返す。
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type userRepository struct{ q db.Querier }

func NewUserRepository(q db.Querier) UserRepository { return &userRepository{q: q} }

func (r *userRepository) Create(ctx context.Context, p CreateUserParams) (*domain.User, error) {
	row, err := r.q.CreateUser(ctx, db.CreateUserParams{Email: p.Email, PasswordHash: p.PasswordHash, Name: p.Name})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return toUser(row), nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, mapFindErr("find user by email", err)
	}
	return toUser(row), nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, mapFindErr("find user by id", err)
	}
	return toUser(row), nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	if err := r.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: id, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func toUser(u db.User) *domain.User {
	return &domain.User{
		ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, Name: u.Name,
		EmailVerified: u.EmailVerified, Role: domain.Role(u.Role),
		CreatedAt: u.CreatedAt.Time, UpdatedAt: u.UpdatedAt.Time,
	}
}

func mapFindErr(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
