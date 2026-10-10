package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

var jst = time.FixedZone("JST", 9*60*60)

// clinicEnv は clinic-reservation のクエリ検証用の環境。並行テストのため pool を使う。
type clinicEnv struct {
	ctx  context.Context
	pool *pgxpool.Pool
	q    *db.Queries

	patient1, patient2 db.User
	deptID, doctorID   int64
}

func newClinicEnv(t *testing.T) *clinicEnv {
	t.Helper()
	ctx := context.Background()
	pool := pgtest.NewPool(t)
	e := &clinicEnv{ctx: ctx, pool: pool, q: db.New(pool)}
	e.patient1 = e.user(t, "p1@example.com", "患者1")
	e.patient2 = e.user(t, "p2@example.com", "患者2")
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO departments(name, standard_consult_minutes) VALUES ('内科', 10) RETURNING id`).Scan(&e.deptID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO doctors(department_id, name) VALUES ($1, '山田') RETURNING id`, e.deptID).Scan(&e.doctorID))
	return e
}

func (e *clinicEnv) user(t *testing.T, email, name string) db.User {
	t.Helper()
	u, err := e.q.CreateUser(e.ctx, db.CreateUserParams{Email: email, PasswordHash: "h", Name: name})
	require.NoError(t, err)
	return u
}

// slot は capacity 付きの枠を 1 つ作って返す。
func (e *clinicEnv) slot(t *testing.T, start time.Time, capacity int32) db.Slot {
	t.Helper()
	require.NoError(t, e.q.CreateSlotIfAbsent(e.ctx, db.CreateSlotIfAbsentParams{
		DepartmentID: e.deptID, DoctorID: e.doctorID, StartAt: ts(start), Capacity: capacity,
	}))
	slots, err := e.q.ListSlots(e.ctx, db.ListSlotsParams{
		DepartmentID: e.deptID, FromAt: ts(start), ToAt: ts(start.Add(time.Second)),
	})
	require.NoError(t, err)
	require.Len(t, slots, 1)
	return slots[0]
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func date(t time.Time) pgtype.Date      { return pgtype.Date{Time: t, Valid: true} }
func puuid(id uuid.UUID) pgtype.UUID    { return pgtype.UUID{Bytes: id, Valid: true} }
