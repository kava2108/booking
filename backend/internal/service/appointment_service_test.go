package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 12-clinic-reservation-appointment-api TASK-0001

var (
	patient  = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	otherPat = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	slotID   = uuid.MustParse("77777777-7777-7777-7777-777777777777")
	slot2ID  = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	apptID   = uuid.MustParse("99999999-9999-9999-9999-999999999999")
)

func detail(id uuid.UUID, start time.Time) db.GetSlotDetailRow {
	return db.GetSlotDetailRow{ID: id, DepartmentID: 1, DoctorID: 7, StartAt: tsOf(start), Capacity: 2, Status: "open", DepartmentName: "内科", DoctorName: "山田"}
}

func newApptService(e *clinicEnv, numbers ...string) service.AppointmentService {
	opts := []service.ClinicOption{service.WithClinicClock(e.now)}
	if len(numbers) > 0 {
		i := 0
		opts = append(opts, service.WithAppointmentNumbers(func(time.Time) string {
			n := numbers[min(i, len(numbers)-1)]
			i++
			return n
		}))
	}
	return service.NewAppointmentService(e.repos, e.tx, opts...)
}

func expectSettings(e *clinicEnv, deadline, window int32) {
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: deadline, BookingWindowDays: window}, nil).Maybe()
}

func booked(id, slot uuid.UUID, owner uuid.UUID) db.Appointment {
	return db.Appointment{ID: id, AppointmentNo: "A-20261019-0001", PatientID: owner, SlotID: slot, VisitType: "revisit", SymptomNote: "咳", Status: "booked"}
}

func lastNotice(t *testing.T, p db.EnqueueNotificationParams) domain.AppointmentNotice {
	t.Helper()
	var n domain.AppointmentNotice
	require.NoError(t, json.Unmarshal(p.Payload, &n))
	return n
}

// ---------- Create (REQ-002-AC-1/2/5) ----------

func TestCreate_Success(t *testing.T) {
	e := newClinicEnv(t)
	start := day(19, 9, 30) // 来週月曜
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, start), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, db.CountPatientBookingsAtParams{PatientID: patient, StartAt: tsOf(start)}).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{ID: slotID, BookedCount: 1}, nil)
	var created db.CreateAppointmentParams
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.CreateAppointmentParams) (db.Appointment, error) {
			created = p
			return db.Appointment{ID: apptID, AppointmentNo: p.AppointmentNo, PatientID: p.PatientID, SlotID: p.SlotID, Status: "booked"}, nil
		})
	var notice db.EnqueueNotificationParams
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationParams) (db.Notification, error) {
			notice = p
			return db.Notification{}, nil
		})

	res, err := newApptService(e, "A-20261019-0042").Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "revisit", SymptomNote: "3日前から咳"})
	require.NoError(t, err)

	assert.Equal(t, apptID, res.ID)
	assert.Equal(t, "A-20261019-0042", res.AppointmentNo)
	assert.Equal(t, "内科", res.Department)
	assert.Equal(t, "山田", res.Doctor)
	assert.True(t, res.SlotStart.Equal(start))
	assert.Equal(t, db.CreateAppointmentParams{AppointmentNo: "A-20261019-0042", PatientID: patient, SlotID: slotID, VisitType: "revisit", SymptomNote: "3日前から咳"}, created)

	// 確認通知は同じ Tx でアウトボックスに登録。症状メモは含まれない（REQ-007）
	assert.Equal(t, patient, notice.UserID)
	assert.Equal(t, domain.KindAppointmentConfirmed, notice.Kind)
	n := lastNotice(t, notice)
	assert.Equal(t, "A-20261019-0042", n.AppointmentNo)
	assert.Equal(t, "内科", n.Department)
	assert.NotContains(t, string(notice.Payload), "咳")
	assert.NotContains(t, string(notice.Payload), "symptom")
}

// 予約番号の形式: A-<診療日(JST)>-<4 桁>
func TestCreate_DefaultAppointmentNumberFormat(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	var no string
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.CreateAppointmentParams) (db.Appointment, error) {
			no = p.AppointmentNo
			return db.Appointment{ID: apptID, AppointmentNo: no}, nil
		})
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, nil)

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	require.NoError(t, err)
	assert.Regexp(t, `^A-20261019-\d{4}$`, no)
}

// REQ-002-AC-2: 満席 → 409。枠確保に失敗したら予約も通知も作らない
func TestCreate_SlotFull(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, pgx.ErrNoRows)
	// CreateAppointment / EnqueueNotification は期待していない（呼ばれたら失敗）

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	assert.ErrorIs(t, err, service.ErrSlotFull)
}

// REQ-002-AC-5: 同じ時刻に有効な予約がある → 重複（枠は確保しない）
func TestCreate_DuplicateAtSameTime(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(1), nil)

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	assert.ErrorIs(t, err, service.ErrDuplicateAppointment)
}

// 同時リクエストで事前チェックをすり抜けても、DB の一意制約で重複と判定する
func TestCreate_UniqueViolationOnPatientSlotIsDuplicate(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).Return(db.Appointment{}, &pgconn.PgError{Code: "23505", ConstraintName: repository.ConstraintActivePatientSlot})

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	assert.ErrorIs(t, err, service.ErrDuplicateAppointment)
}

// 予約番号が衝突したら番号を変えて Tx ごと再試行する
func TestCreate_RetriesOnAppointmentNumberCollision(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	var tried []string
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.CreateAppointmentParams) (db.Appointment, error) {
			tried = append(tried, p.AppointmentNo)
			if len(tried) < 3 {
				return db.Appointment{}, &pgconn.PgError{Code: "23505", ConstraintName: repository.ConstraintAppointmentNo}
			}
			return db.Appointment{ID: apptID, AppointmentNo: p.AppointmentNo}, nil
		})
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, nil).Once()

	res, err := newApptService(e, "N1", "N2", "N3").Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	require.NoError(t, err)
	assert.Equal(t, []string{"N1", "N2", "N3"}, tried)
	assert.Equal(t, "N3", res.AppointmentNo)
}

func TestCreate_GivesUpAfterRepeatedCollisions(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).
		Return(db.Appointment{}, &pgconn.PgError{Code: "23505", ConstraintName: repository.ConstraintAppointmentNo}).Times(5)

	_, err := newApptService(e, "N").Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, service.ErrDuplicateAppointment)
}

func TestCreate_SlotNotBookable(t *testing.T) {
	cases := map[string]struct {
		d      db.GetSlotDetailRow
		reason string
	}{
		"存在しない枠":     {db.GetSlotDetailRow{}, "unknown"},
		"休診(closed)": {func() db.GetSlotDetailRow { d := detail(slotID, day(19, 9, 30)); d.Status = "closed"; return d }(), "closed"},
		"過去の枠":       {detail(slotID, day(12, 8, 0)), "past_cutoff"},
		"現在ちょうどの枠":   {detail(slotID, day(12, 9, 0)), "past_cutoff"},
		"受付期間外":      {detail(slotID, day(12, 9, 0).AddDate(0, 0, 31)), "outside_window"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t)
			expectSettings(e, 60, 30)
			if c.reason == "unknown" {
				e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(db.GetSlotDetailRow{}, pgx.ErrNoRows)
			} else {
				e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(c.d, nil)
			}
			// ReserveSlot 以降は呼ばれない

			_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
			var ve *service.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, "slot_id", ve.Fields[0].Field)
		})
	}
}

// 境界値: 受付期間の最終日の枠は予約できる
func TestCreate_LastDayOfBookingWindowIsBookable(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	last := day(12, 17, 0).AddDate(0, 0, 30)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, last), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).Return(db.Appointment{ID: apptID}, nil)
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, nil)

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	assert.NoError(t, err)
}

func TestCreate_RepositoryErrorsPropagate(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	boom := errors.New("db down")
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(db.GetSlotDetailRow{}, boom)

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	assert.ErrorIs(t, err, boom)
}

// 通知の登録に失敗したら予約全体を失敗にする（Tx が巻き戻る）
func TestCreate_NotificationFailureFailsTheTransaction(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	boom := errors.New("db down")
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).Return(db.Appointment{ID: apptID}, nil)
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, boom)

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first"})
	assert.ErrorIs(t, err, boom)
}

// ---------- ListMine (REQ-003-AC-1) ----------

func TestListMine_CancelableFlag(t *testing.T) {
	e := newClinicEnv(t) // 現在 10/12 09:00、キャンセル期限 60 分
	expectSettings(e, 60, 30)
	rows := []db.ListAppointmentsByPatientRow{
		{ID: uuid.New(), AppointmentNo: "A-1", Status: "booked", VisitType: "first", SymptomNote: "咳", StartAt: tsOf(day(12, 9, 59)), DepartmentName: "内科", DoctorName: "山田"}, // 期限(開始-60分=8:59)を過ぎている
		{ID: uuid.New(), AppointmentNo: "A-2", Status: "booked", VisitType: "first", StartAt: tsOf(day(12, 10, 0)), DepartmentName: "内科", DoctorName: "山田"},                   // 期限ちょうど(9:00) → 不可
		{ID: uuid.New(), AppointmentNo: "A-3", Status: "booked", VisitType: "revisit", StartAt: tsOf(day(12, 10, 1)), DepartmentName: "内科", DoctorName: "山田"},                 // 期限前 → 可
		{ID: uuid.New(), AppointmentNo: "A-4", Status: "canceled", VisitType: "first", StartAt: tsOf(day(20, 10, 0)), DepartmentName: "内科", DoctorName: "山田"},                 // キャンセル済み → 不可
		{ID: uuid.New(), AppointmentNo: "A-5", Status: "completed", VisitType: "first", StartAt: tsOf(day(5, 10, 0)), DepartmentName: "内科", DoctorName: "山田"},                 // 過去
	}
	e.appts.EXPECT().ListAppointmentsByPatient(mock.Anything, patient).Return(rows, nil)

	got, err := newApptService(e).ListMine(context.Background(), patient)
	require.NoError(t, err)

	require.Len(t, got, 5)
	assert.Equal(t, []bool{false, false, true, false, false}, []bool{got[0].Cancelable, got[1].Cancelable, got[2].Cancelable, got[3].Cancelable, got[4].Cancelable})
	assert.Equal(t, "咳", got[0].SymptomNote, "本人の一覧には症状メモを含む")
	assert.Equal(t, "内科", got[0].Department)
	assert.Equal(t, "completed", got[4].Status)
}

func TestListMine_EmptyIsNotNil(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.appts.EXPECT().ListAppointmentsByPatient(mock.Anything, patient).Return(nil, nil)

	got, err := newApptService(e).ListMine(context.Background(), patient)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// ---------- Cancel (REQ-003-AC-2/3) ----------

func expectCancelLookup(e *clinicEnv, start time.Time) {
	expectSettings(e, 60, 30)
	e.appts.EXPECT().GetAppointment(mock.Anything, db.GetAppointmentParams{ID: apptID, PatientID: patient}).Return(booked(apptID, slotID, patient), nil)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, start), nil)
}

func TestCancel_Success(t *testing.T) {
	e := newClinicEnv(t)
	start := day(19, 9, 30)
	expectCancelLookup(e, start)
	e.appts.EXPECT().CancelAppointment(mock.Anything, db.CancelAppointmentParams{ID: apptID, PatientID: patient}).Return(db.Appointment{ID: apptID, Status: "canceled"}, nil)
	e.slots.EXPECT().ReleaseSlot(mock.Anything, slotID).Return(nil)
	var notice db.EnqueueNotificationParams
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationParams) (db.Notification, error) {
			notice = p
			return db.Notification{}, nil
		})

	require.NoError(t, newApptService(e).Cancel(context.Background(), patient, apptID))

	assert.Equal(t, domain.KindAppointmentCanceled, notice.Kind)
	assert.Equal(t, patient, notice.UserID)
	assert.NotContains(t, string(notice.Payload), "咳")
}

// 境界値: 開始 60 分前ちょうどは不可、1ns 前は可
func TestCancel_DeadlineBoundary(t *testing.T) {
	deadline := day(19, 9, 30).Add(-60 * time.Minute)

	t.Run("期限ちょうどは不可", func(t *testing.T) {
		e := newClinicEnv(t)
		*e.clock = deadline
		expectCancelLookup(e, day(19, 9, 30))
		err := newApptService(e).Cancel(context.Background(), patient, apptID)
		assert.ErrorIs(t, err, service.ErrCancelDeadlinePassed)
	})
	t.Run("期限の直前は可", func(t *testing.T) {
		e := newClinicEnv(t)
		*e.clock = deadline.Add(-time.Nanosecond)
		expectCancelLookup(e, day(19, 9, 30))
		e.appts.EXPECT().CancelAppointment(mock.Anything, mock.Anything).Return(db.Appointment{}, nil)
		e.slots.EXPECT().ReleaseSlot(mock.Anything, slotID).Return(nil)
		e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, nil)
		assert.NoError(t, newApptService(e).Cancel(context.Background(), patient, apptID))
	})
	t.Run("開始後は不可", func(t *testing.T) {
		e := newClinicEnv(t)
		*e.clock = day(19, 9, 31)
		expectCancelLookup(e, day(19, 9, 30))
		assert.ErrorIs(t, newApptService(e).Cancel(context.Background(), patient, apptID), service.ErrCancelDeadlinePassed)
	})
}

func TestCancel_NotFoundCases(t *testing.T) {
	t.Run("他人の予約・存在しない", func(t *testing.T) {
		e := newClinicEnv(t)
		e.appts.EXPECT().GetAppointment(mock.Anything, db.GetAppointmentParams{ID: apptID, PatientID: otherPat}).Return(db.Appointment{}, pgx.ErrNoRows)
		assert.ErrorIs(t, newApptService(e).Cancel(context.Background(), otherPat, apptID), service.ErrAppointmentNotFound)
	})
	t.Run("キャンセル済み・完了済み", func(t *testing.T) {
		for _, status := range []string{"canceled", "completed", "no_show"} {
			e := newClinicEnv(t)
			a := booked(apptID, slotID, patient)
			a.Status = status
			e.appts.EXPECT().GetAppointment(mock.Anything, mock.Anything).Return(a, nil)
			assert.ErrorIs(t, newApptService(e).Cancel(context.Background(), patient, apptID), service.ErrAppointmentNotFound, status)
		}
	})
	t.Run("競合で先に取り消された", func(t *testing.T) {
		e := newClinicEnv(t)
		expectCancelLookup(e, day(19, 9, 30))
		e.appts.EXPECT().CancelAppointment(mock.Anything, mock.Anything).Return(db.Appointment{}, pgx.ErrNoRows)
		// ReleaseSlot は呼ばれない（二重解放しない）
		assert.ErrorIs(t, newApptService(e).Cancel(context.Background(), patient, apptID), service.ErrAppointmentNotFound)
	})
}

// ---------- Reschedule (REQ-003-AC-4) ----------

func expectRescheduleLookup(e *clinicEnv, oldStart, newStart time.Time) {
	expectSettings(e, 60, 30)
	e.appts.EXPECT().GetAppointment(mock.Anything, db.GetAppointmentParams{ID: apptID, PatientID: patient}).Return(booked(apptID, slotID, patient), nil)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, oldStart), nil)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slot2ID).Return(detail(slot2ID, newStart), nil)
}

func TestReschedule_Success(t *testing.T) {
	e := newClinicEnv(t)
	oldStart, newStart := day(19, 9, 30), day(26, 10, 0)
	expectRescheduleLookup(e, oldStart, newStart)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, db.CountPatientBookingsAtParams{PatientID: patient, StartAt: tsOf(newStart)}).Return(int64(0), nil)
	var order []string
	e.slots.EXPECT().ReserveSlot(mock.Anything, slot2ID).RunAndReturn(func(context.Context, uuid.UUID) (db.Slot, error) {
		order = append(order, "reserve-new")
		return db.Slot{}, nil
	})
	e.appts.EXPECT().RescheduleAppointment(mock.Anything, db.RescheduleAppointmentParams{ID: apptID, PatientID: patient, SlotID: slot2ID}).RunAndReturn(
		func(context.Context, db.RescheduleAppointmentParams) (db.Appointment, error) {
			order = append(order, "move")
			a := booked(apptID, slot2ID, patient)
			return a, nil
		})
	e.slots.EXPECT().ReleaseSlot(mock.Anything, slotID).RunAndReturn(func(context.Context, uuid.UUID) error { order = append(order, "release-old"); return nil })
	var notice db.EnqueueNotificationParams
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationParams) (db.Notification, error) {
			notice = p
			return db.Notification{}, nil
		})

	res, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
	require.NoError(t, err)

	assert.Equal(t, []string{"reserve-new", "move", "release-old"}, order, "新枠確保 → 付替え → 旧枠解放（同一 Tx）")
	assert.True(t, res.SlotStart.Equal(newStart))
	assert.Equal(t, domain.KindAppointmentRescheduled, notice.Kind)
	n := lastNotice(t, notice)
	require.NotNil(t, n.PreviousStartAt)
	assert.True(t, n.PreviousStartAt.Equal(oldStart))
	assert.True(t, n.StartAt.Equal(newStart))
}

func TestReschedule_NewSlotFull(t *testing.T) {
	e := newClinicEnv(t)
	expectRescheduleLookup(e, day(19, 9, 30), day(26, 10, 0))
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slot2ID).Return(db.Slot{}, pgx.ErrNoRows)
	// 付替え・旧枠解放は呼ばれない（旧予約は維持される）

	_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
	assert.ErrorIs(t, err, service.ErrSlotFull)
}

func TestReschedule_RejectsSameSlot(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	e.appts.EXPECT().GetAppointment(mock.Anything, mock.Anything).Return(booked(apptID, slotID, patient), nil)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil).Maybe()

	_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slotID)
	var ve *service.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "slot_id", ve.Fields[0].Field)
}

// 変更も取消と同じ期限が適用される（元の予約の開始が近い場合は変更不可）
func TestReschedule_OldAppointmentPastDeadline(t *testing.T) {
	e := newClinicEnv(t)
	*e.clock = day(19, 8, 45) // 元の予約 9:30 の 45 分前（期限 60 分）
	expectSettings(e, 60, 30)
	e.appts.EXPECT().GetAppointment(mock.Anything, mock.Anything).Return(booked(apptID, slotID, patient), nil)
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)

	_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
	assert.ErrorIs(t, err, service.ErrCancelDeadlinePassed)
}

func TestReschedule_DuplicateAtNewTime(t *testing.T) {
	e := newClinicEnv(t)
	newStart := day(26, 10, 0)
	expectRescheduleLookup(e, day(19, 9, 30), newStart)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, db.CountPatientBookingsAtParams{PatientID: patient, StartAt: tsOf(newStart)}).Return(int64(1), nil)

	_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
	assert.ErrorIs(t, err, service.ErrDuplicateAppointment)
}

// 同じ時刻の別医師の枠への変更は、自分自身の予約を重複とみなさない
func TestReschedule_SameTimeOtherDoctorIsNotDuplicateOfItself(t *testing.T) {
	e := newClinicEnv(t)
	start := day(19, 9, 30)
	expectRescheduleLookup(e, start, start)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(1), nil) // 自分の予約のみ
	e.slots.EXPECT().ReserveSlot(mock.Anything, slot2ID).Return(db.Slot{}, nil)
	e.appts.EXPECT().RescheduleAppointment(mock.Anything, mock.Anything).Return(booked(apptID, slot2ID, patient), nil)
	e.slots.EXPECT().ReleaseSlot(mock.Anything, slotID).Return(nil)
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, nil)

	_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
	assert.NoError(t, err)
}

func TestReschedule_NotFoundAndNotBookable(t *testing.T) {
	t.Run("他人の予約", func(t *testing.T) {
		e := newClinicEnv(t)
		e.appts.EXPECT().GetAppointment(mock.Anything, db.GetAppointmentParams{ID: apptID, PatientID: otherPat}).Return(db.Appointment{}, pgx.ErrNoRows)
		_, err := newApptService(e).Reschedule(context.Background(), otherPat, apptID, slot2ID)
		assert.ErrorIs(t, err, service.ErrAppointmentNotFound)
	})
	t.Run("新しい枠が休診", func(t *testing.T) {
		e := newClinicEnv(t)
		expectSettings(e, 60, 30)
		closed := detail(slot2ID, day(26, 10, 0))
		closed.Status = "closed"
		e.appts.EXPECT().GetAppointment(mock.Anything, mock.Anything).Return(booked(apptID, slotID, patient), nil)
		e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
		e.slots.EXPECT().GetSlotDetail(mock.Anything, slot2ID).Return(closed, nil)
		_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
		var ve *service.ValidationError
		assert.ErrorAs(t, err, &ve)
	})
	t.Run("競合で予約が取り消された", func(t *testing.T) {
		e := newClinicEnv(t)
		expectRescheduleLookup(e, day(19, 9, 30), day(26, 10, 0))
		e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
		e.slots.EXPECT().ReserveSlot(mock.Anything, slot2ID).Return(db.Slot{}, nil)
		e.appts.EXPECT().RescheduleAppointment(mock.Anything, mock.Anything).Return(db.Appointment{}, pgx.ErrNoRows)
		_, err := newApptService(e).Reschedule(context.Background(), patient, apptID, slot2ID)
		assert.ErrorIs(t, err, service.ErrAppointmentNotFound)
	})
}

// ---------- 通知本文の保護 (REQ-007-AC-2) ----------

func TestNoticePayloadNeverContainsSymptomNote(t *testing.T) {
	e := newClinicEnv(t)
	expectSettings(e, 60, 30)
	secret := "糖尿病の疑い・要検査"
	e.slots.EXPECT().GetSlotDetail(mock.Anything, slotID).Return(detail(slotID, day(19, 9, 30)), nil)
	e.appts.EXPECT().CountPatientBookingsAt(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.slots.EXPECT().ReserveSlot(mock.Anything, slotID).Return(db.Slot{}, nil)
	e.appts.EXPECT().CreateAppointment(mock.Anything, mock.Anything).Return(db.Appointment{ID: apptID, SymptomNote: secret}, nil)
	var payload string
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationParams) (db.Notification, error) {
			payload = string(p.Payload)
			return db.Notification{}, nil
		})

	_, err := newApptService(e).Create(context.Background(), patient, service.CreateAppointmentInput{SlotID: slotID, VisitType: "first", SymptomNote: secret})
	require.NoError(t, err)
	assert.False(t, strings.Contains(payload, secret) || strings.Contains(payload, "糖尿病"))
}
