package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// IMP 5-clinic-reservation-sqlc-repository TASK-0001: トランザクション境界（予約確定の原子性、REQ-002-AC-2）

func newStore(t *testing.T) (*repository.Store, *pgxpool.Pool, db.Slot, db.User) {
	t.Helper()
	ctx := context.Background()
	pool := pgtest.NewPool(t)

	q := db.New(pool)
	u, err := q.CreateUser(ctx, db.CreateUserParams{Email: "p@example.com", PasswordHash: "h", Name: "患者"})
	require.NoError(t, err)
	var deptID, docID int64
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO departments(name) VALUES ('内科') RETURNING id`).Scan(&deptID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO doctors(department_id,name) VALUES ($1,'山田') RETURNING id`, deptID).Scan(&docID))
	start := pgtype.Timestamptz{Time: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC), Valid: true}
	require.NoError(t, q.CreateSlotIfAbsent(ctx, db.CreateSlotIfAbsentParams{DepartmentID: deptID, DoctorID: docID, StartAt: start, Capacity: 2}))
	slots, err := q.ListSlots(ctx, db.ListSlotsParams{DepartmentID: deptID, FromAt: start, ToAt: pgtype.Timestamptz{Time: start.Time.Add(time.Hour), Valid: true}})
	require.NoError(t, err)
	require.Len(t, slots, 1)
	return repository.NewStore(pool), pool, slots[0], u
}

func TestWithinTx_CommitsOnSuccess(t *testing.T) {
	s, pool, slot, user := newStore(t)
	ctx := context.Background()

	err := s.WithinTx(ctx, func(r repository.Repositories) error {
		if _, err := r.Slots.ReserveSlot(ctx, slot.ID); err != nil {
			return err
		}
		_, err := r.Appointments.CreateAppointment(ctx, db.CreateAppointmentParams{AppointmentNo: "A-1", PatientID: user.ID, SlotID: slot.ID, VisitType: "first"})
		return err
	})
	require.NoError(t, err)

	got, _ := db.New(pool).GetSlot(ctx, slot.ID)
	assert.EqualValues(t, 1, got.BookedCount)
}

// 予約作成が失敗したら枠の確保も巻き戻る（二重予約でカウントだけ増えない）
func TestWithinTx_RollsBackReserveWhenAppointmentFails(t *testing.T) {
	s, pool, slot, user := newStore(t)
	ctx := context.Background()
	book := func(no string) error {
		return s.WithinTx(ctx, func(r repository.Repositories) error {
			if _, err := r.Slots.ReserveSlot(ctx, slot.ID); err != nil {
				return err
			}
			_, err := r.Appointments.CreateAppointment(ctx, db.CreateAppointmentParams{AppointmentNo: no, PatientID: user.ID, SlotID: slot.ID, VisitType: "first"})
			return err
		})
	}
	require.NoError(t, book("A-1"))
	require.Error(t, book("A-2"), "同一患者・同一枠の二重予約は失敗")

	got, _ := db.New(pool).GetSlot(ctx, slot.ID)
	assert.EqualValues(t, 1, got.BookedCount, "失敗した試行の確保は残らない")
}

func TestWithinTx_ReturnsCallbackErrorUnwrapped(t *testing.T) {
	s, _, slot, _ := newStore(t)
	sentinel := errors.New("boom")

	err := s.WithinTx(context.Background(), func(r repository.Repositories) error {
		_, _ = r.Slots.ReserveSlot(context.Background(), slot.ID)
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
}

func TestWithinTx_RollsBackOnPanicAndRepanics(t *testing.T) {
	s, pool, slot, _ := newStore(t)
	ctx := context.Background()

	assert.Panics(t, func() {
		_ = s.WithinTx(ctx, func(r repository.Repositories) error {
			_, _ = r.Slots.ReserveSlot(ctx, slot.ID)
			panic("unexpected")
		})
	})
	got, _ := db.New(pool).GetSlot(ctx, slot.ID)
	assert.EqualValues(t, 0, got.BookedCount)
}

// pgx.ErrNoRows（満席）はそのまま呼び出し側で判定できる
func TestWithinTx_PropagatesNoRows(t *testing.T) {
	s, _, slot, _ := newStore(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		require.NoError(t, s.WithinTx(ctx, func(r repository.Repositories) error {
			_, err := r.Slots.ReserveSlot(ctx, slot.ID)
			return err
		}))
	}
	err := s.WithinTx(ctx, func(r repository.Repositories) error {
		_, err := r.Slots.ReserveSlot(ctx, slot.ID)
		return err
	})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// 非 Tx 経路でも同じ interface で使える
func TestStore_ExposesRepositoriesOutsideTx(t *testing.T) {
	s, _, slot, _ := newStore(t)

	got, err := s.Repositories().Slots.GetSlot(context.Background(), slot.ID)
	require.NoError(t, err)
	assert.Equal(t, slot.ID, got.ID)
}
