package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 13-clinic-reservation-queue-staff-walkin TASK-0001

var (
	entryID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	deptID  = int64(1)
)

func newQueueService(e *clinicEnv) service.QueueService {
	return service.NewQueueService(e.repos, e.tx, service.WithClinicClock(e.now))
}

func today() pgtype.Date { return pgtype.Date{Time: domain.StartOfDay(clinicNow), Valid: true} }

func apptRow(status string, start time.Time) db.GetAppointmentWithSlotRow {
	return db.GetAppointmentWithSlotRow{ID: apptID, AppointmentNo: "A-1", PatientID: patient, Status: status, SlotID: slotID, DepartmentID: deptID, StartAt: tsOf(start)}
}

func entry(status string) db.QueueEntry {
	return db.QueueEntry{ID: entryID, DepartmentID: deptID, Status: status, Source: "walk_in", TicketNo: 1}
}

// ---------- 来院受付 (REQ-005-AC-1) ----------

func expectLockAndTicket(e *clinicEnv, ticket int32) {
	e.admin.EXPECT().LockDepartment(mock.Anything, deptID).Return(deptID, nil)
	e.queue.EXPECT().NextTicketNo(mock.Anything, db.NextTicketNoParams{DepartmentID: deptID, ServiceDate: today()}).Return(ticket, nil)
}

func TestCheckIn_EarlyArrivalIsOrderedByAppointmentTime(t *testing.T) {
	e := newClinicEnv(t) // 現在 9:00
	start := day(12, 9, 30)
	e.appts.EXPECT().GetAppointmentWithSlot(mock.Anything, apptID).Return(apptRow("booked", start), nil)
	expectLockAndTicket(e, 4)
	var got db.EnqueueEntryParams
	e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueEntryParams) (db.QueueEntry, error) {
			got = p
			return db.QueueEntry{ID: entryID, Status: "waiting"}, nil
		})

	st, err := newQueueService(e).CheckIn(context.Background(), apptID)
	require.NoError(t, err)

	assert.Equal(t, entryID, st.ID)
	assert.Equal(t, "waiting", st.Status)
	assert.Equal(t, deptID, got.DepartmentID)
	assert.Equal(t, puuid(apptID), got.AppointmentID)
	assert.Equal(t, puuid(patient), got.PatientID)
	assert.False(t, got.DisplayName.Valid)
	assert.Equal(t, "reservation", got.Source)
	assert.Equal(t, today(), got.ServiceDate)
	assert.EqualValues(t, 4, got.TicketNo)
	assert.True(t, got.SortTime.Time.Equal(start), "早く着いても順番は予約時刻")
	assert.True(t, got.CheckedInAt.Time.Equal(clinicNow))
}

// 遅刻した予約患者は、先に受付した当日患者を追い越さない（sort_time = 受付時刻）
func TestCheckIn_LateArrivalIsOrderedByArrivalTime(t *testing.T) {
	e := newClinicEnv(t) // 現在 9:00、予約は 8:30
	e.appts.EXPECT().GetAppointmentWithSlot(mock.Anything, apptID).Return(apptRow("booked", day(12, 8, 30)), nil)
	expectLockAndTicket(e, 1)
	var got db.EnqueueEntryParams
	e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueEntryParams) (db.QueueEntry, error) {
			got = p
			return db.QueueEntry{ID: entryID, Status: "waiting"}, nil
		})

	_, err := newQueueService(e).CheckIn(context.Background(), apptID)
	require.NoError(t, err)
	assert.True(t, got.SortTime.Time.Equal(clinicNow))
}

func TestCheckIn_Rejections(t *testing.T) {
	cases := map[string]struct {
		row db.GetAppointmentWithSlotRow
		err error
	}{
		"取消済み":    {apptRow("canceled", day(12, 9, 30)), nil},
		"完了済み":    {apptRow("completed", day(12, 9, 30)), nil},
		"翌日の予約":   {apptRow("booked", day(13, 9, 30)), nil},
		"前日の予約":   {apptRow("booked", day(11, 9, 30)), nil},
		"存在しない予約": {db.GetAppointmentWithSlotRow{}, pgx.ErrNoRows},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t) // LockDepartment 以降は呼ばれない
			e.appts.EXPECT().GetAppointmentWithSlot(mock.Anything, apptID).Return(c.row, c.err)

			_, err := newQueueService(e).CheckIn(context.Background(), apptID)
			if c.err != nil {
				assert.ErrorIs(t, err, service.ErrAppointmentNotFound)
				return
			}
			var ve *service.ValidationError
			assert.ErrorAs(t, err, &ve)
		})
	}
}

// 境界: 当日 0:00 と 23:59 の予約は当日扱い
func TestCheckIn_DayBoundaries(t *testing.T) {
	for _, start := range []time.Time{day(12, 0, 0), day(12, 23, 59)} {
		e := newClinicEnv(t)
		e.appts.EXPECT().GetAppointmentWithSlot(mock.Anything, apptID).Return(apptRow("booked", start), nil)
		expectLockAndTicket(e, 1)
		e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).Return(db.QueueEntry{ID: entryID, Status: "waiting"}, nil)

		_, err := newQueueService(e).CheckIn(context.Background(), apptID)
		assert.NoError(t, err, start.String())
	}
}

func TestCheckIn_AlreadyCheckedIn(t *testing.T) {
	e := newClinicEnv(t)
	e.appts.EXPECT().GetAppointmentWithSlot(mock.Anything, apptID).Return(apptRow("booked", day(12, 9, 30)), nil)
	expectLockAndTicket(e, 2)
	e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).Return(db.QueueEntry{}, &pgconn.PgError{Code: "23505", ConstraintName: repository.ConstraintQueueAppointment})

	_, err := newQueueService(e).CheckIn(context.Background(), apptID)
	assert.ErrorIs(t, err, service.ErrAlreadyInQueue)
}

func TestCheckIn_RetriesOnTicketCollision(t *testing.T) {
	e := newClinicEnv(t)
	e.appts.EXPECT().GetAppointmentWithSlot(mock.Anything, apptID).Return(apptRow("booked", day(12, 9, 30)), nil)
	e.admin.EXPECT().LockDepartment(mock.Anything, deptID).Return(deptID, nil)
	n := int32(0)
	e.queue.EXPECT().NextTicketNo(mock.Anything, mock.Anything).RunAndReturn(func(context.Context, db.NextTicketNoParams) (int32, error) { n++; return n, nil })
	calls := 0
	e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p db.EnqueueEntryParams) (db.QueueEntry, error) {
		calls++
		if calls < 3 {
			return db.QueueEntry{}, &pgconn.PgError{Code: "23505", ConstraintName: repository.ConstraintQueueTicket}
		}
		assert.EqualValues(t, 3, p.TicketNo, "採番し直して再試行")
		return db.QueueEntry{ID: entryID, Status: "waiting"}, nil
	})

	_, err := newQueueService(e).CheckIn(context.Background(), apptID)
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

// ---------- 呼出・開始・完了 (REQ-005-AC-2) ----------

func TestTransitions_Success(t *testing.T) {
	cases := map[string]struct {
		call   func(e *clinicEnv, svc service.QueueService) (*service.QueueState, error)
		expect func(e *clinicEnv)
		status string
	}{
		"call": {
			func(e *clinicEnv, s service.QueueService) (*service.QueueState, error) {
				return s.Call(context.Background(), entryID)
			},
			func(e *clinicEnv) {
				e.queue.EXPECT().CallQueueEntry(mock.Anything, entryID).Return(entry("called"), nil)
			}, "called",
		},
		"start": {
			func(e *clinicEnv, s service.QueueService) (*service.QueueState, error) {
				return s.Start(context.Background(), entryID)
			},
			func(e *clinicEnv) {
				e.queue.EXPECT().StartQueueEntry(mock.Anything, entryID).Return(entry("in_consultation"), nil)
			}, "in_consultation",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t)
			c.expect(e)
			st, err := c.call(e, newQueueService(e))
			require.NoError(t, err)
			assert.Equal(t, c.status, st.Status)
			assert.Equal(t, entryID, st.ID)
		})
	}
}

// 不正な遷移は 0 行更新 → 存在すれば ErrInvalidTransition（現在の状態つき）、無ければ NotFound
func TestTransitions_InvalidAndMissing(t *testing.T) {
	t.Run("不正遷移", func(t *testing.T) {
		e := newClinicEnv(t)
		e.queue.EXPECT().StartQueueEntry(mock.Anything, entryID).Return(db.QueueEntry{}, pgx.ErrNoRows)
		e.queue.EXPECT().GetQueueEntry(mock.Anything, entryID).Return(entry("waiting"), nil)

		_, err := newQueueService(e).Start(context.Background(), entryID)
		var it *service.InvalidTransitionError
		require.ErrorAs(t, err, &it)
		assert.Equal(t, "waiting", it.Current)
		assert.ErrorIs(t, err, service.ErrInvalidTransition)
	})
	t.Run("存在しない", func(t *testing.T) {
		e := newClinicEnv(t)
		e.queue.EXPECT().CallQueueEntry(mock.Anything, entryID).Return(db.QueueEntry{}, pgx.ErrNoRows)
		e.queue.EXPECT().GetQueueEntry(mock.Anything, entryID).Return(db.QueueEntry{}, pgx.ErrNoRows)

		_, err := newQueueService(e).Call(context.Background(), entryID)
		assert.ErrorIs(t, err, service.ErrQueueEntryNotFound)
	})
}

// 完了: 予約由来なら予約も completed にする（同一 Tx）。当日受付（予約なし）は触らない
func TestComplete_MarksReservationCompleted(t *testing.T) {
	e := newClinicEnv(t)
	done := entry("done")
	done.AppointmentID = puuid(apptID)
	done.Source = "reservation"
	e.queue.EXPECT().CompleteQueueEntry(mock.Anything, entryID).Return(done, nil)
	e.appts.EXPECT().CompleteAppointment(mock.Anything, apptID).Return(int64(1), nil)

	st, err := newQueueService(e).Complete(context.Background(), entryID)
	require.NoError(t, err)
	assert.Equal(t, "done", st.Status)
}

func TestComplete_WalkInDoesNotTouchAppointments(t *testing.T) {
	e := newClinicEnv(t) // CompleteAppointment は期待していない
	e.queue.EXPECT().CompleteQueueEntry(mock.Anything, entryID).Return(entry("done"), nil)

	_, err := newQueueService(e).Complete(context.Background(), entryID)
	assert.NoError(t, err)
}

func TestComplete_AppointmentUpdateFailureFailsTheTransaction(t *testing.T) {
	e := newClinicEnv(t)
	boom := errors.New("db down")
	done := entry("done")
	done.AppointmentID = puuid(apptID)
	e.queue.EXPECT().CompleteQueueEntry(mock.Anything, entryID).Return(done, nil)
	e.appts.EXPECT().CompleteAppointment(mock.Anything, apptID).Return(int64(0), boom)

	_, err := newQueueService(e).Complete(context.Background(), entryID)
	assert.ErrorIs(t, err, boom)
}

// ---------- 当日受付 (REQ-008) ----------

func deptWith(cutoff *time.Duration, capacity *int32) db.Department {
	d := db.Department{ID: deptID, Name: "内科"}
	if cutoff != nil {
		d.WalkinCutoff = pgtype.Time{Microseconds: cutoff.Microseconds(), Valid: true}
	}
	if capacity != nil {
		d.WalkinCapacity = pgtype.Int4{Int32: *capacity, Valid: true}
	}
	return d
}

func dur(h, m int) *time.Duration {
	d := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
	return &d
}
func i32(v int32) *int32 { return &v }

func expectWalkInFlow(e *clinicEnv, d db.Department, existingWalkIns int64) {
	e.admin.EXPECT().GetDepartment(mock.Anything, deptID).Return(d, nil)
	e.admin.EXPECT().LockDepartment(mock.Anything, deptID).Return(deptID, nil)
	e.queue.EXPECT().CountWalkInsForDate(mock.Anything, db.CountWalkInsForDateParams{DepartmentID: deptID, ServiceDate: today()}).Return(existingWalkIns, nil).Maybe()
}

func TestCreateWalkIn_PatientSuccess(t *testing.T) {
	e := newClinicEnv(t)
	expectWalkInFlow(e, deptWith(nil, nil), 0)
	e.queue.EXPECT().CountActiveQueueEntriesForPatient(mock.Anything, db.CountActiveQueueEntriesForPatientParams{DepartmentID: deptID, ServiceDate: today(), PatientID: puuid(patient)}).Return(int64(0), nil)
	e.queue.EXPECT().NextTicketNo(mock.Anything, mock.Anything).Return(int32(7), nil)
	var got db.EnqueueEntryParams
	e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueEntryParams) (db.QueueEntry, error) {
			got = p
			return db.QueueEntry{ID: entryID, TicketNo: p.TicketNo, SortTime: p.SortTime}, nil
		})
	e.queue.EXPECT().CountAhead(mock.Anything, db.CountAheadParams{DepartmentID: deptID, ServiceDate: today(), SortTime: tsOf(clinicNow)}).Return(int64(3), nil)

	tk, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, PatientID: &patient})
	require.NoError(t, err)

	assert.Equal(t, entryID, tk.QueueEntryID)
	assert.Equal(t, 7, tk.TicketNo)
	assert.Equal(t, 3, tk.AheadCount)
	assert.Equal(t, "walk_in", got.Source)
	assert.False(t, got.AppointmentID.Valid)
	assert.Equal(t, puuid(patient), got.PatientID)
	assert.True(t, got.SortTime.Time.Equal(clinicNow), "当日受付は受付時刻順")
}

func TestCreateWalkIn_StaffForAccountlessPatient(t *testing.T) {
	e := newClinicEnv(t)
	expectWalkInFlow(e, deptWith(nil, nil), 0)
	e.queue.EXPECT().NextTicketNo(mock.Anything, mock.Anything).Return(int32(1), nil)
	var got db.EnqueueEntryParams
	e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueEntryParams) (db.QueueEntry, error) {
			got = p
			return db.QueueEntry{ID: entryID, TicketNo: 1}, nil
		})
	e.queue.EXPECT().CountAhead(mock.Anything, mock.Anything).Return(int64(0), nil)

	_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, DisplayName: "  電話 太郎 "})
	require.NoError(t, err)
	assert.False(t, got.PatientID.Valid)
	assert.Equal(t, pgtype.Text{String: "電話 太郎", Valid: true}, got.DisplayName, "前後の空白は除く")
}

// 境界値: 締切時刻ちょうどは拒否、1 分前は可
func TestCreateWalkIn_CutoffBoundary(t *testing.T) {
	t.Run("締切ちょうど", func(t *testing.T) {
		e := newClinicEnv(t)
		expectWalkInFlow(e, deptWith(dur(9, 0), nil), 0) // 現在 9:00
		_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, DisplayName: "x"})
		var wc *service.WalkInClosedError
		require.ErrorAs(t, err, &wc)
		assert.Equal(t, "after_cutoff", wc.Reason)
		assert.ErrorIs(t, err, service.ErrWalkInClosed)
	})
	t.Run("締切の 1 分前", func(t *testing.T) {
		e := newClinicEnv(t)
		expectWalkInFlow(e, deptWith(dur(9, 1), nil), 0)
		e.queue.EXPECT().NextTicketNo(mock.Anything, mock.Anything).Return(int32(1), nil)
		e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).Return(db.QueueEntry{ID: entryID}, nil)
		e.queue.EXPECT().CountAhead(mock.Anything, mock.Anything).Return(int64(0), nil)
		_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, DisplayName: "x"})
		assert.NoError(t, err)
	})
}

// 境界値: 定員ちょうどに達していれば拒否、あと 1 人なら可
func TestCreateWalkIn_CapacityBoundary(t *testing.T) {
	t.Run("定員に達している", func(t *testing.T) {
		e := newClinicEnv(t)
		expectWalkInFlow(e, deptWith(nil, i32(20)), 20)
		_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, DisplayName: "x"})
		var wc *service.WalkInClosedError
		require.ErrorAs(t, err, &wc)
		assert.Equal(t, "capacity_reached", wc.Reason)
	})
	t.Run("最後の 1 人は可", func(t *testing.T) {
		e := newClinicEnv(t)
		expectWalkInFlow(e, deptWith(nil, i32(20)), 19)
		e.queue.EXPECT().NextTicketNo(mock.Anything, mock.Anything).Return(int32(20), nil)
		e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).Return(db.QueueEntry{ID: entryID}, nil)
		e.queue.EXPECT().CountAhead(mock.Anything, mock.Anything).Return(int64(0), nil)
		_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, DisplayName: "x"})
		assert.NoError(t, err)
	})
}

func TestCreateWalkIn_PatientAlreadyInQueue(t *testing.T) {
	e := newClinicEnv(t)
	expectWalkInFlow(e, deptWith(nil, nil), 0)
	e.queue.EXPECT().CountActiveQueueEntriesForPatient(mock.Anything, mock.Anything).Return(int64(1), nil)

	_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, PatientID: &patient})
	assert.ErrorIs(t, err, service.ErrAlreadyInQueue)
}

func TestCreateWalkIn_Validation(t *testing.T) {
	cases := map[string]service.WalkInInput{
		"患者も氏名も無い": {DepartmentID: deptID},
		"氏名が空白のみ":  {DepartmentID: deptID, DisplayName: "   "},
		"氏名が長すぎる":  {DepartmentID: deptID, DisplayName: strings.Repeat("あ", 101)},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t) // repository は一切呼ばれない
			_, err := newQueueService(e).CreateWalkIn(context.Background(), in)
			var ve *service.ValidationError
			assert.ErrorAs(t, err, &ve)
		})
	}
	t.Run("存在しない診療科", func(t *testing.T) {
		e := newClinicEnv(t)
		e.admin.EXPECT().GetDepartment(mock.Anything, int64(99)).Return(db.Department{}, pgx.ErrNoRows)
		_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: 99, DisplayName: "x"})
		var ve *service.ValidationError
		require.ErrorAs(t, err, &ve)
		assert.Equal(t, "department_id", ve.Fields[0].Field)
	})
	t.Run("氏名ちょうど 100 文字は可", func(t *testing.T) {
		e := newClinicEnv(t)
		expectWalkInFlow(e, deptWith(nil, nil), 0)
		e.queue.EXPECT().NextTicketNo(mock.Anything, mock.Anything).Return(int32(1), nil)
		e.queue.EXPECT().EnqueueEntry(mock.Anything, mock.Anything).Return(db.QueueEntry{ID: entryID}, nil)
		e.queue.EXPECT().CountAhead(mock.Anything, mock.Anything).Return(int64(0), nil)
		_, err := newQueueService(e).CreateWalkIn(context.Background(), service.WalkInInput{DepartmentID: deptID, DisplayName: strings.Repeat("あ", 100)})
		assert.NoError(t, err)
	})
}

// ---------- スタッフ向け待機列 (REQ-005-AC-1, REQ-007-AC-2) ----------

func TestListStaffQueue(t *testing.T) {
	e := newClinicEnv(t)
	e.queue.EXPECT().ListStaffQueue(mock.Anything, db.ListStaffQueueParams{DepartmentID: deptID, ServiceDate: today()}).Return([]db.ListStaffQueueRow{
		{ID: entryID, TicketNo: 1, Source: "reservation", Status: "waiting", PatientName: "患者1", SymptomNote: "咳", CheckedInAt: tsOf(day(12, 8, 55))},
		{ID: uuid.New(), TicketNo: 2, Source: "walk_in", Status: "called", PatientName: "電話 太郎"},
	}, nil)

	got, err := newQueueService(e).ListStaffQueue(context.Background(), deptID)
	require.NoError(t, err)

	require.Len(t, got, 2)
	assert.Equal(t, "患者1", got[0].PatientName)
	assert.Equal(t, "咳", got[0].SymptomNote)
	assert.Equal(t, "walk_in", got[1].Source)
}

func TestListStaffQueue_EmptyIsNotNil(t *testing.T) {
	e := newClinicEnv(t)
	e.queue.EXPECT().ListStaffQueue(mock.Anything, mock.Anything).Return(nil, nil)

	got, err := newQueueService(e).ListStaffQueue(context.Background(), deptID)
	require.NoError(t, err)
	assert.NotNil(t, got)
}
