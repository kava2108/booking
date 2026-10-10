package db_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository/db"
)

// IMP 5-clinic-reservation-sqlc-repository TASK-0001

var day = time.Date(2026, 10, 12, 0, 0, 0, 0, jst) // 月曜

func at(h, m int) time.Time {
	return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

// ---------- slots (REQ-001, REQ-002-AC-2) ----------

func TestListSlots_FiltersByDoctorAndRange(t *testing.T) {
	e := newClinicEnv(t)
	e.slot(t, at(9, 0), 1)
	e.slot(t, at(9, 30), 1)
	e.slot(t, at(14, 0), 1)

	all, err := e.q.ListSlots(e.ctx, db.ListSlotsParams{DepartmentID: e.deptID, FromAt: ts(at(0, 0)), ToAt: ts(at(23, 59))})
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.True(t, all[0].StartAt.Time.Before(all[1].StartAt.Time), "時刻順")

	morning, err := e.q.ListSlots(e.ctx, db.ListSlotsParams{DepartmentID: e.deptID, FromAt: ts(at(0, 0)), ToAt: ts(at(12, 0))})
	require.NoError(t, err)
	assert.Len(t, morning, 2)

	byDoctor, err := e.q.ListSlots(e.ctx, db.ListSlotsParams{
		DepartmentID: e.deptID, DoctorID: pgtype.Int8{Int64: e.doctorID, Valid: true}, FromAt: ts(at(0, 0)), ToAt: ts(at(23, 59)),
	})
	require.NoError(t, err)
	assert.Len(t, byDoctor, 3)

	other, err := e.q.ListSlots(e.ctx, db.ListSlotsParams{
		DepartmentID: e.deptID, DoctorID: pgtype.Int8{Int64: e.doctorID + 99, Valid: true}, FromAt: ts(at(0, 0)), ToAt: ts(at(23, 59)),
	})
	require.NoError(t, err)
	assert.Empty(t, other)
}

func TestCreateSlotIfAbsent_Idempotent(t *testing.T) {
	e := newClinicEnv(t)
	e.slot(t, at(9, 0), 2)
	require.NoError(t, e.q.CreateSlotIfAbsent(e.ctx, db.CreateSlotIfAbsentParams{
		DepartmentID: e.deptID, DoctorID: e.doctorID, StartAt: ts(at(9, 0)), Capacity: 5,
	}))
	slots, err := e.q.ListSlots(e.ctx, db.ListSlotsParams{DepartmentID: e.deptID, FromAt: ts(at(0, 0)), ToAt: ts(at(23, 0))})
	require.NoError(t, err)
	require.Len(t, slots, 1)
	assert.EqualValues(t, 2, slots[0].Capacity, "既存枠は上書きしない")
}

func TestReserveSlot_UntilFull(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 2)

	for i := 1; i <= 2; i++ {
		got, err := e.q.ReserveSlot(e.ctx, s.ID)
		require.NoError(t, err)
		assert.EqualValues(t, i, got.BookedCount)
	}
	_, err := e.q.ReserveSlot(e.ctx, s.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows, "満席は 0 行更新")
}

func TestReserveSlot_ClosedSlotRejected(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 2)
	_, err := e.q.CloseSlots(e.ctx, db.CloseSlotsParams{FromAt: ts(at(0, 0)), ToAt: ts(at(23, 0))})
	require.NoError(t, err)

	_, err = e.q.ReserveSlot(e.ctx, s.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// 境界値・並行: capacity=1 に 20 並行で成功は 1 件のみ（REQ-002-AC-2）
func TestReserveSlot_ConcurrentNeverOverbooks(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 1)

	var ok, full atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.q.ReserveSlot(e.ctx, s.ID)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, pgx.ErrNoRows):
				full.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()

	assert.EqualValues(t, 1, ok.Load())
	assert.EqualValues(t, 19, full.Load())
	got, err := e.q.GetSlot(e.ctx, s.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, got.BookedCount)
}

func TestReleaseSlot_NeverGoesNegative(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 2)
	_, err := e.q.ReserveSlot(e.ctx, s.ID)
	require.NoError(t, err)

	require.NoError(t, e.q.ReleaseSlot(e.ctx, s.ID))
	require.NoError(t, e.q.ReleaseSlot(e.ctx, s.ID)) // 余分な解放は無視
	got, err := e.q.GetSlot(e.ctx, s.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, got.BookedCount)
}

// REQ-005-AC-3: 休診時間帯の枠だけを closed にする
func TestCloseSlots_OnlyWithinRange(t *testing.T) {
	e := newClinicEnv(t)
	morning := e.slot(t, at(9, 0), 1)
	afternoon := e.slot(t, at(14, 0), 1)

	ids, err := e.q.CloseSlots(e.ctx, db.CloseSlotsParams{
		DoctorID: pgtype.Int8{Int64: e.doctorID, Valid: true}, FromAt: ts(at(8, 0)), ToAt: ts(at(12, 0)),
	})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{morning.ID}, ids)

	a, _ := e.q.GetSlot(e.ctx, afternoon.ID)
	assert.Equal(t, "open", a.Status)
	m, _ := e.q.GetSlot(e.ctx, morning.ID)
	assert.Equal(t, "closed", m.Status)
}

// ---------- appointments (REQ-002, REQ-003) ----------

func (e *clinicEnv) book(t *testing.T, patient db.User, s db.Slot, no string) db.Appointment {
	t.Helper()
	a, err := e.q.CreateAppointment(e.ctx, db.CreateAppointmentParams{
		AppointmentNo: no, PatientID: patient.ID, SlotID: s.ID, VisitType: "revisit", SymptomNote: "咳",
	})
	require.NoError(t, err)
	return a
}

func TestAppointment_CreateAndListByPatient(t *testing.T) {
	e := newClinicEnv(t)
	s1, s2 := e.slot(t, at(9, 0), 1), e.slot(t, at(10, 0), 1)
	e.book(t, e.patient1, s2, "A-2")
	e.book(t, e.patient1, s1, "A-1")
	e.book(t, e.patient2, s1, "A-3")

	rows, err := e.q.ListAppointmentsByPatient(e.ctx, e.patient1.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "他人の予約は含まない")
	assert.Equal(t, "A-1", rows[0].AppointmentNo, "予約日時の昇順")
	assert.Equal(t, "内科", rows[0].DepartmentName)
	assert.Equal(t, "山田", rows[0].DoctorName)
}

func TestAppointment_DuplicateBookingRejected(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 2)
	e.book(t, e.patient1, s, "A-1")

	_, err := e.q.CreateAppointment(e.ctx, db.CreateAppointmentParams{AppointmentNo: "A-2", PatientID: e.patient1.ID, SlotID: s.ID, VisitType: "first"})
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23505", pgErr.Code)
}

// REQ-002-AC-5: 同一時刻に別枠の予約があれば検出できる
func TestCountPatientBookingsAt(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 1)
	e.book(t, e.patient1, s, "A-1")

	n, err := e.q.CountPatientBookingsAt(e.ctx, db.CountPatientBookingsAtParams{PatientID: e.patient1.ID, StartAt: ts(at(9, 0))})
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	n, _ = e.q.CountPatientBookingsAt(e.ctx, db.CountPatientBookingsAtParams{PatientID: e.patient2.ID, StartAt: ts(at(9, 0))})
	assert.EqualValues(t, 0, n)
}

// セキュリティ(IDOR): 他人の予約は取得・キャンセル・変更できない
func TestAppointment_ScopedToPatient(t *testing.T) {
	e := newClinicEnv(t)
	s, s2 := e.slot(t, at(9, 0), 2), e.slot(t, at(10, 0), 2)
	a := e.book(t, e.patient1, s, "A-1")

	_, err := e.q.GetAppointment(e.ctx, db.GetAppointmentParams{ID: a.ID, PatientID: e.patient2.ID})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = e.q.CancelAppointment(e.ctx, db.CancelAppointmentParams{ID: a.ID, PatientID: e.patient2.ID})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = e.q.RescheduleAppointment(e.ctx, db.RescheduleAppointmentParams{ID: a.ID, PatientID: e.patient2.ID, SlotID: s2.ID})
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	got, err := e.q.GetAppointment(e.ctx, db.GetAppointmentParams{ID: a.ID, PatientID: e.patient1.ID})
	require.NoError(t, err)
	assert.Equal(t, "booked", got.Status)
}

func TestAppointment_CancelThenRebook(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 1)
	a := e.book(t, e.patient1, s, "A-1")

	c, err := e.q.CancelAppointment(e.ctx, db.CancelAppointmentParams{ID: a.ID, PatientID: e.patient1.ID})
	require.NoError(t, err)
	assert.Equal(t, "canceled", c.Status)
	assert.True(t, c.CanceledAt.Valid)

	_, err = e.q.CancelAppointment(e.ctx, db.CancelAppointmentParams{ID: a.ID, PatientID: e.patient1.ID})
	assert.ErrorIs(t, err, pgx.ErrNoRows, "二重キャンセルは不可")

	e.book(t, e.patient1, s, "A-2") // キャンセル後は同じ枠に再予約できる
}

func TestAppointment_Reschedule(t *testing.T) {
	e := newClinicEnv(t)
	s1, s2 := e.slot(t, at(9, 0), 1), e.slot(t, at(10, 0), 1)
	a := e.book(t, e.patient1, s1, "A-1")

	r, err := e.q.RescheduleAppointment(e.ctx, db.RescheduleAppointmentParams{ID: a.ID, PatientID: e.patient1.ID, SlotID: s2.ID})
	require.NoError(t, err)
	assert.Equal(t, s2.ID, r.SlotID)
}

// REQ-005-AC-3: 休診で影響を受ける予約者を通知用に取得
func TestListBookedAppointmentsForNotice(t *testing.T) {
	e := newClinicEnv(t)
	s1, s2 := e.slot(t, at(9, 0), 2), e.slot(t, at(14, 0), 2)
	e.book(t, e.patient1, s1, "A-1")
	e.book(t, e.patient2, s2, "A-2")
	canceled := e.book(t, e.patient2, s1, "A-3")
	_, err := e.q.CancelAppointment(e.ctx, db.CancelAppointmentParams{ID: canceled.ID, PatientID: e.patient2.ID})
	require.NoError(t, err)

	rows, err := e.q.ListBookedAppointmentsForNotice(e.ctx, db.ListBookedAppointmentsForNoticeParams{FromAt: ts(at(8, 0)), ToAt: ts(at(12, 0))})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "A-1", rows[0].AppointmentNo)
	assert.Equal(t, e.patient1.ID, rows[0].PatientID)
}

// ---------- queue (REQ-005, 008, 004) ----------

func (e *clinicEnv) enqueueWalkIn(t *testing.T, name string, sortAt time.Time) db.QueueEntry {
	t.Helper()
	no, err := e.q.NextTicketNo(e.ctx, db.NextTicketNoParams{DepartmentID: e.deptID, ServiceDate: date(day)})
	require.NoError(t, err)
	q, err := e.q.EnqueueEntry(e.ctx, db.EnqueueEntryParams{
		DepartmentID: e.deptID, DisplayName: pgtype.Text{String: name, Valid: true}, Source: "walk_in",
		ServiceDate: date(day), TicketNo: no, SortTime: ts(sortAt), CheckedInAt: ts(sortAt),
	})
	require.NoError(t, err)
	return q
}

func TestNextTicketNo_IncrementsPerDayAndDepartment(t *testing.T) {
	e := newClinicEnv(t)
	n1, err := e.q.NextTicketNo(e.ctx, db.NextTicketNoParams{DepartmentID: e.deptID, ServiceDate: date(day)})
	require.NoError(t, err)
	assert.EqualValues(t, 1, n1)

	e.enqueueWalkIn(t, "A", at(9, 0))
	e.enqueueWalkIn(t, "B", at(9, 5))
	n3, _ := e.q.NextTicketNo(e.ctx, db.NextTicketNoParams{DepartmentID: e.deptID, ServiceDate: date(day)})
	assert.EqualValues(t, 3, n3)

	next, _ := e.q.NextTicketNo(e.ctx, db.NextTicketNoParams{DepartmentID: e.deptID, ServiceDate: date(day.AddDate(0, 0, 1))})
	assert.EqualValues(t, 1, next, "翌日は 1 から")
}

// 並行採番は UNIQUE 違反で検出できる（service がリトライする前提）
func TestEnqueue_TicketCollisionDetected(t *testing.T) {
	e := newClinicEnv(t)
	e.enqueueWalkIn(t, "A", at(9, 0))

	_, err := e.q.EnqueueEntry(e.ctx, db.EnqueueEntryParams{
		DepartmentID: e.deptID, DisplayName: pgtype.Text{String: "B", Valid: true}, Source: "walk_in",
		ServiceDate: date(day), TicketNo: 1, SortTime: ts(at(9, 1)), CheckedInAt: ts(at(9, 1)),
	})
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23505", pgErr.Code)
}

func TestQueueTransitions_ValidAndInvalid(t *testing.T) {
	e := newClinicEnv(t)
	q := e.enqueueWalkIn(t, "A", at(9, 0))
	assert.Equal(t, "waiting", q.Status)

	// 不正遷移は 0 行（→ 409）
	_, err := e.q.StartQueueEntry(e.ctx, q.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows, "waiting から直接開始は不可")
	_, err = e.q.CompleteQueueEntry(e.ctx, q.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows, "waiting から直接完了は不可")

	called, err := e.q.CallQueueEntry(e.ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, "called", called.Status)
	_, err = e.q.CallQueueEntry(e.ctx, q.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows, "二重呼出は不可")

	started, err := e.q.StartQueueEntry(e.ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, "in_consultation", started.Status)
	assert.True(t, started.StartedAt.Valid)

	done, err := e.q.CompleteQueueEntry(e.ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, "done", done.Status)
	assert.True(t, done.CompletedAt.Valid)
}

func TestQueueCancel_OnlyWhileActive(t *testing.T) {
	e := newClinicEnv(t)
	q := e.enqueueWalkIn(t, "A", at(9, 0))
	c, err := e.q.CancelQueueEntry(e.ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, "canceled", c.Status)
	_, err = e.q.CancelQueueEntry(e.ctx, q.ID)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// REQ-004-AC-2/4: 自分より前の人数（受付順）。完了・キャンセル済みは数えない
func TestCountAheadAndWaiting(t *testing.T) {
	e := newClinicEnv(t)
	a := e.enqueueWalkIn(t, "A", at(9, 0))
	b := e.enqueueWalkIn(t, "B", at(9, 5))
	c := e.enqueueWalkIn(t, "C", at(9, 10))

	ahead := func(q db.QueueEntry) int64 {
		n, err := e.q.CountAhead(e.ctx, db.CountAheadParams{DepartmentID: e.deptID, ServiceDate: date(day), SortTime: q.SortTime})
		require.NoError(t, err)
		return n
	}
	waiting := func() int64 {
		n, err := e.q.CountWaiting(e.ctx, db.CountWaitingParams{DepartmentID: e.deptID, ServiceDate: date(day)})
		require.NoError(t, err)
		return n
	}
	assert.EqualValues(t, 0, ahead(a))
	assert.EqualValues(t, 1, ahead(b))
	assert.EqualValues(t, 2, ahead(c))
	assert.EqualValues(t, 3, waiting())

	_, _ = e.q.CallQueueEntry(e.ctx, a.ID)
	_, _ = e.q.StartQueueEntry(e.ctx, a.ID)
	assert.EqualValues(t, 1, ahead(b), "診察中も前にいる人として数える")
	_, _ = e.q.CompleteQueueEntry(e.ctx, a.ID)
	assert.EqualValues(t, 0, ahead(b), "完了は数えない")
	_, _ = e.q.CancelQueueEntry(e.ctx, b.ID)
	assert.EqualValues(t, 0, ahead(c))
	assert.EqualValues(t, 1, waiting())
}

// REQ-004-AC-2: 直近の診察所要時間（秒）。新しい順に limit 件
func TestRecentConsultSeconds(t *testing.T) {
	e := newClinicEnv(t)
	for i, d := range []time.Duration{10 * time.Minute, 20 * time.Minute, 30 * time.Minute} {
		q := e.enqueueWalkIn(t, "P", at(9, i))
		_, err := e.q.CallQueueEntry(e.ctx, q.ID)
		require.NoError(t, err)
		_, err = e.q.StartQueueEntry(e.ctx, q.ID)
		require.NoError(t, err)
		// 所要時間を固定するため時刻を直接設定
		_, err = e.pool.Exec(e.ctx, `UPDATE queue_entries SET started_at=$2, completed_at=$3, status='done' WHERE id=$1`,
			q.ID, at(10, i*40), at(10, i*40).Add(d))
		require.NoError(t, err)
	}
	e.enqueueWalkIn(t, "未完了", at(11, 0))

	secs, err := e.q.RecentConsultSeconds(e.ctx, db.RecentConsultSecondsParams{DepartmentID: e.deptID, ServiceDate: date(day), Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, []float64{1800, 1200}, secs, "完了時刻の新しい順に 2 件")
}

func TestCountOnDutyDoctors(t *testing.T) {
	e := newClinicEnv(t)
	_, err := e.pool.Exec(e.ctx, `INSERT INTO doctor_schedules(doctor_id,weekday,start_time,end_time,slot_minutes,capacity) VALUES ($1,1,'09:00','12:00',30,1)`, e.doctorID)
	require.NoError(t, err)

	count := func(at time.Time) int64 {
		n, err := e.q.CountOnDutyDoctors(e.ctx, db.CountOnDutyDoctorsParams{DepartmentID: e.deptID, At: ts(at)})
		require.NoError(t, err)
		return n
	}
	assert.EqualValues(t, 1, count(at(10, 0)), "診療時間内（月曜 JST）")
	assert.EqualValues(t, 0, count(at(8, 59)), "開始前")
	assert.EqualValues(t, 0, count(at(12, 0)), "終了時刻ちょうどは時間外")
	assert.EqualValues(t, 0, count(at(10, 0).AddDate(0, 0, 1)), "火曜は診療なし")

	require.NoError(t, e.q.CreateClosure(e.ctx, db.CreateClosureParams{
		DoctorID: pgtype.Int8{Int64: e.doctorID, Valid: true}, FromAt: ts(at(9, 0)), ToAt: ts(at(12, 0)), Reason: "学会",
	}))
	assert.EqualValues(t, 0, count(at(10, 0)), "休診中は数えない")

	_, err = e.pool.Exec(e.ctx, `UPDATE doctors SET active=false WHERE id=$1`, e.doctorID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, count(at(15, 0)))
}

func TestStaffQueue_ShowsNamesAndPublicCountsDoNot(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 1)
	a := e.book(t, e.patient1, s, "A-1")
	// 予約患者の来院受付
	_, err := e.q.EnqueueEntry(e.ctx, db.EnqueueEntryParams{
		DepartmentID: e.deptID, AppointmentID: puuid(a.ID), PatientID: puuid(e.patient1.ID), Source: "reservation",
		ServiceDate: date(day), TicketNo: 1, SortTime: ts(at(9, 0)), CheckedInAt: ts(at(8, 55)),
	})
	require.NoError(t, err)
	e.enqueueWalkIn(t, "電話 太郎", at(9, 10))

	rows, err := e.q.ListStaffQueue(e.ctx, db.ListStaffQueueParams{DepartmentID: e.deptID, ServiceDate: date(day)})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "患者1", rows[0].PatientName, "アカウントがあれば users.name")
	assert.Equal(t, "咳", rows[0].SymptomNote)
	assert.Equal(t, "電話 太郎", rows[1].PatientName, "無アカウントは display_name")
}

// 患者本人の順番参照は appointment_id + patient_id で絞る（IDOR 対策）
func TestGetQueueEntryForPatient_Scoped(t *testing.T) {
	e := newClinicEnv(t)
	s := e.slot(t, at(9, 0), 1)
	a := e.book(t, e.patient1, s, "A-1")
	_, err := e.q.EnqueueEntry(e.ctx, db.EnqueueEntryParams{
		DepartmentID: e.deptID, AppointmentID: puuid(a.ID), PatientID: puuid(e.patient1.ID), Source: "reservation",
		ServiceDate: date(day), TicketNo: 1, SortTime: ts(at(9, 0)), CheckedInAt: ts(at(8, 55)),
	})
	require.NoError(t, err)

	got, err := e.q.GetQueueEntryForPatient(e.ctx, db.GetQueueEntryForPatientParams{AppointmentID: puuid(a.ID), PatientID: puuid(e.patient1.ID)})
	require.NoError(t, err)
	assert.EqualValues(t, 1, got.TicketNo)

	_, err = e.q.GetQueueEntryForPatient(e.ctx, db.GetQueueEntryForPatientParams{AppointmentID: puuid(a.ID), PatientID: puuid(e.patient2.ID)})
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// ---------- notifications (REQ-002-AC-3) ----------

func TestNotification_ClaimLeaseAndMark(t *testing.T) {
	e := newClinicEnv(t)
	n, err := e.q.EnqueueNotification(e.ctx, db.EnqueueNotificationParams{UserID: e.patient1.ID, Kind: "appointment_confirmed", Payload: []byte(`{"appointment_no":"A-1"}`)})
	require.NoError(t, err)
	assert.Equal(t, "pending", n.Status)

	claimed, err := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 10, LeaseSeconds: 300})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.EqualValues(t, 1, claimed[0].Attempts)

	again, err := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 10, LeaseSeconds: 300})
	require.NoError(t, err)
	assert.Empty(t, again, "リース中は再取得されない")

	require.NoError(t, e.q.MarkNotificationSent(e.ctx, n.ID))
	_, err = e.pool.Exec(e.ctx, `UPDATE notifications SET send_after = now() - interval '1 second'`)
	require.NoError(t, err)
	again, _ = e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 10, LeaseSeconds: 300})
	assert.Empty(t, again, "送信済みは対象外")
}

func TestNotification_FutureSendAfterNotClaimed(t *testing.T) {
	e := newClinicEnv(t)
	n, err := e.q.EnqueueNotification(e.ctx, db.EnqueueNotificationParams{UserID: e.patient1.ID, Kind: "reminder", Payload: []byte(`{}`)})
	require.NoError(t, err)
	_, err = e.pool.Exec(e.ctx, `UPDATE notifications SET send_after = now() + interval '1 hour' WHERE id=$1`, n.ID)
	require.NoError(t, err)

	claimed, err := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 10, LeaseSeconds: 60})
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestNotification_LeaseExpiryAllowsRetryAndFailedExcluded(t *testing.T) {
	e := newClinicEnv(t)
	n, _ := e.q.EnqueueNotification(e.ctx, db.EnqueueNotificationParams{UserID: e.patient1.ID, Kind: "k", Payload: []byte(`{}`)})
	_, err := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 1, LeaseSeconds: 300})
	require.NoError(t, err)

	_, err = e.pool.Exec(e.ctx, `UPDATE notifications SET send_after = now() - interval '1 second'`)
	require.NoError(t, err)
	retry, err := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 1, LeaseSeconds: 300})
	require.NoError(t, err)
	require.Len(t, retry, 1)
	assert.EqualValues(t, 2, retry[0].Attempts, "リース切れ後の再取得で attempts が増える")

	require.NoError(t, e.q.MarkNotificationFailed(e.ctx, n.ID))
	_, _ = e.pool.Exec(e.ctx, `UPDATE notifications SET send_after = now() - interval '1 second'`)
	none, _ := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 1, LeaseSeconds: 300})
	assert.Empty(t, none)
}

// 複数ワーカーが同時に取得しても重複しない（SKIP LOCKED）
func TestNotification_ConcurrentClaimNoDuplicates(t *testing.T) {
	e := newClinicEnv(t)
	for i := 0; i < 30; i++ {
		_, err := e.q.EnqueueNotification(e.ctx, db.EnqueueNotificationParams{UserID: e.patient1.ID, Kind: "k", Payload: []byte(`{}`)})
		require.NoError(t, err)
	}
	var mu sync.Mutex
	seen := map[uuid.UUID]int{}
	var wg sync.WaitGroup
	for w := 0; w < 6; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := e.q.ClaimNotifications(e.ctx, db.ClaimNotificationsParams{Limit: 10, LeaseSeconds: 300})
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			mu.Lock()
			for _, r := range rows {
				seen[r.ID]++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	for id, c := range seen {
		assert.Equal(t, 1, c, "通知 %s が重複取得された", id)
	}
	assert.Len(t, seen, 30)
}

// ---------- admin (REQ-006) ----------

func TestAdminSettingsAndSchedules(t *testing.T) {
	e := newClinicEnv(t)

	cs, err := e.q.GetClinicSettings(e.ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 60, cs.CancelDeadlineMinutes)

	cs, err = e.q.UpdateClinicSettings(e.ctx, db.UpdateClinicSettingsParams{CancelDeadlineMinutes: 120, BookingWindowDays: 14})
	require.NoError(t, err)
	assert.EqualValues(t, 14, cs.BookingWindowDays)

	_, err = e.q.UpdateClinicSettings(e.ctx, db.UpdateClinicSettingsParams{CancelDeadlineMinutes: -1, BookingWindowDays: 14})
	assert.Error(t, err, "負のキャンセル期限は CHECK で拒否")

	d, err := e.q.UpdateDepartmentSettings(e.ctx, db.UpdateDepartmentSettingsParams{ID: e.deptID, StandardConsultMinutes: pgtype.Int4{Int32: 15, Valid: true}})
	require.NoError(t, err)
	assert.EqualValues(t, 15, d.StandardConsultMinutes.Int32)

	require.NoError(t, e.q.DeleteDoctorSchedules(e.ctx, e.doctorID))
	require.NoError(t, e.q.CreateDoctorSchedule(e.ctx, db.CreateDoctorScheduleParams{
		DoctorID: e.doctorID, Weekday: 1, StartTime: pgtype.Time{Microseconds: 9 * 3600 * 1e6, Valid: true},
		EndTime: pgtype.Time{Microseconds: 12 * 3600 * 1e6, Valid: true}, SlotMinutes: 30, Capacity: 2,
	}))
	sch, err := e.q.ListSchedulesByWeekday(e.ctx, 1)
	require.NoError(t, err)
	require.Len(t, sch, 1)
	assert.EqualValues(t, 2, sch[0].Capacity)
	assert.Equal(t, e.deptID, sch[0].DepartmentID, "枠生成に診療科 ID が必要")
}

func TestListDepartmentsAndDoctors(t *testing.T) {
	e := newClinicEnv(t)
	deps, err := e.q.ListDepartments(e.ctx)
	require.NoError(t, err)
	assert.Len(t, deps, 1)
	docs, err := e.q.ListDoctors(e.ctx, e.deptID)
	require.NoError(t, err)
	assert.Len(t, docs, 1)

	_, err = e.pool.Exec(e.ctx, `UPDATE doctors SET active=false`)
	require.NoError(t, err)
	docs, _ = e.q.ListDoctors(e.ctx, e.deptID)
	assert.Empty(t, docs, "非アクティブな医師は一覧に出さない")
}

// REQ-007-AC-3: 監査ログ（本文は記録しない）
func TestInsertAuditLog(t *testing.T) {
	e := newClinicEnv(t)
	require.NoError(t, e.q.InsertAuditLog(e.ctx, db.InsertAuditLogParams{ActorID: puuid(e.patient1.ID), Action: "view_symptom_note", Target: "appointment:A-1"}))
	var n int
	require.NoError(t, e.pool.QueryRow(e.ctx, `SELECT count(*) FROM audit_logs`).Scan(&n))
	assert.Equal(t, 1, n)
}
