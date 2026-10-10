package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// IMP 12-clinic-reservation-appointment-api: 一意制約違反の制約名を取り出す（どの制約かで業務上の意味が変わる）

func TestUniqueViolationConstraint(t *testing.T) {
	q := db.New(pgtest.NewPool(t))
	ctx := context.Background()
	_, err := q.CreateUser(ctx, db.CreateUserParams{Email: "a@example.com", PasswordHash: "h", Name: "n"})
	require.NoError(t, err)

	_, err = q.CreateUser(ctx, db.CreateUserParams{Email: "a@example.com", PasswordHash: "h", Name: "n"})
	name, ok := repository.UniqueViolationConstraint(err)
	assert.True(t, ok)
	assert.Equal(t, "users_email_key", name)

	_, ok = repository.UniqueViolationConstraint(nil)
	assert.False(t, ok)
	_, ok = repository.UniqueViolationConstraint(assert.AnError)
	assert.False(t, ok)
}

func TestConstraintNamesMatchMigrations(t *testing.T) {
	pool := pgtest.NewPool(t)
	for _, name := range []string{repository.ConstraintAppointmentNo, repository.ConstraintActivePatientSlot, repository.ConstraintQueueTicket, repository.ConstraintQueueAppointment} {
		var n int
		require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = $1`, name).Scan(&n))
		assert.Equal(t, 1, n, "%s がマイグレーションに存在する", name)
	}
}
