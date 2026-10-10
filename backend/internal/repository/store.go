package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kava2108/booking/backend/internal/repository/db"
)

// Store は pgxpool 上の Repositories と TxRunner を提供する。
type Store struct {
	pool *pgxpool.Pool
	repo Repositories
}

var _ TxRunner = (*Store)(nil)

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, repo: newRepositories(db.New(pool))}
}

// Repositories はトランザクション外（読み取りなど）で使う。
func (s *Store) Repositories() Repositories { return s.repo }

// WithinTx は fn を 1 トランザクションで実行する。
// fn が error を返せば rollback して同じ error を返す（ラップしない。pgx.ErrNoRows 等を呼び出し側で判定できる）。
// panic した場合は rollback して再 panic する。
func (s *Store) WithinTx(ctx context.Context, fn func(Repositories) error) (err error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(newRepositories(db.New(tx))); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
