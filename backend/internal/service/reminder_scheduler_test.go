package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 14-clinic-reservation-notification-outbox TASK-0001: 前日リマインドと「あと N 人」通知（REQ-002-AC-6）

func newScheduler(e *clinicEnv) *service.ReminderScheduler {
	return service.NewReminderScheduler(e.repos, service.WithClinicClock(e.now), service.WithSchedulerLogger(zerolog.Nop()))
}

func reminderRow(no string, start time.Time) db.ListAppointmentsNeedingReminderRow {
	return db.ListAppointmentsNeedingReminderRow{ID: uuid.New(), AppointmentNo: no, PatientID: patient, StartAt: tsOf(start), DepartmentName: "内科", DoctorName: "山田"}
}

// 前日 18:00(JST) 未満はリマインドを探さない
func TestScheduler_NoRemindersBeforeSixPM(t *testing.T) {
	for _, now := range []time.Time{day(12, 0, 0), day(12, 9, 0), day(12, 17, 59)} {
		e := newClinicEnv(t)
		*e.clock = now
		e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, mock.Anything).Return(nil, nil)
		// ListAppointmentsNeedingReminder は期待していない（呼ばれたら失敗）

		res, err := newScheduler(e).RunOnce(context.Background())
		require.NoError(t, err)
		assert.Zero(t, res.Reminders, now.String())
	}
}

// 境界: 18:00 ちょうどから翌日 0:00〜24:00 の予約を対象にする
func TestScheduler_RemindersFromSixPM(t *testing.T) {
	e := newClinicEnv(t)
	*e.clock = day(12, 18, 0)
	tomorrow := day(13, 0, 0)
	rows := []db.ListAppointmentsNeedingReminderRow{reminderRow("A-1", day(13, 9, 30)), reminderRow("A-2", day(13, 15, 0))}
	e.appts.EXPECT().ListAppointmentsNeedingReminder(mock.Anything, db.ListAppointmentsNeedingReminderParams{FromAt: tsOf(tomorrow), ToAt: tsOf(tomorrow.Add(24 * time.Hour))}).Return(rows, nil)
	var enq []db.EnqueueNotificationOnceParams
	e.notif.EXPECT().EnqueueNotificationOnce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationOnceParams) (int64, error) {
			enq = append(enq, p)
			return 1, nil
		})
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, mock.Anything).Return(nil, nil)

	res, err := newScheduler(e).RunOnce(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, res.Reminders)
	require.Len(t, enq, 2)
	assert.Equal(t, domain.KindAppointmentReminder, enq[0].Kind)
	assert.Equal(t, patient, enq[0].UserID)
	var n domain.AppointmentNotice
	require.NoError(t, json.Unmarshal(enq[0].Payload, &n))
	assert.Equal(t, rows[0].ID.String(), n.AppointmentID, "冪等キー（appointment_id）が payload に入る")
	assert.Equal(t, "A-1", n.AppointmentNo)
	assert.Equal(t, "内科", n.Department)
	assert.True(t, n.StartAt.Equal(day(13, 9, 30)))
}

// 既に登録済み（0 件）は数えない。冪等
func TestScheduler_AlreadyEnqueuedRemindersAreNotCounted(t *testing.T) {
	e := newClinicEnv(t)
	*e.clock = day(12, 20, 0)
	e.appts.EXPECT().ListAppointmentsNeedingReminder(mock.Anything, mock.Anything).Return([]db.ListAppointmentsNeedingReminderRow{reminderRow("A-1", day(13, 9, 30))}, nil)
	e.notif.EXPECT().EnqueueNotificationOnce(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, mock.Anything).Return(nil, nil)

	res, err := newScheduler(e).RunOnce(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Reminders)
}

func TestScheduler_ReminderPayloadHasNoSymptom(t *testing.T) {
	e := newClinicEnv(t)
	*e.clock = day(12, 18, 0)
	e.appts.EXPECT().ListAppointmentsNeedingReminder(mock.Anything, mock.Anything).Return([]db.ListAppointmentsNeedingReminderRow{reminderRow("A-1", day(13, 9, 30))}, nil)
	var payload string
	e.notif.EXPECT().EnqueueNotificationOnce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationOnceParams) (int64, error) {
			payload = string(p.Payload)
			return 1, nil
		})
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, mock.Anything).Return(nil, nil)

	_, err := newScheduler(e).RunOnce(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, payload, "symptom")
}

// ---------- あと N 人 ----------

func nearRow(ahead int64, ticket int32) db.ListQueueEntriesNeedingNearNoticeRow {
	return db.ListQueueEntriesNeedingNearNoticeRow{ID: uuid.New(), PatientID: pgtype.UUID{Bytes: patient, Valid: true}, TicketNo: ticket, DepartmentName: "内科", Ahead: ahead}
}

func TestScheduler_QueueNearNotices(t *testing.T) {
	e := newClinicEnv(t) // 現在 9:00（リマインド時間外）
	rows := []db.ListQueueEntriesNeedingNearNoticeRow{nearRow(3, 12), nearRow(1, 13)}
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, db.ListQueueEntriesNeedingNearNoticeParams{ServiceDate: today(), MaxAhead: 3}).Return(rows, nil)
	var enq []db.EnqueueNotificationOnceParams
	e.notif.EXPECT().EnqueueNotificationOnce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationOnceParams) (int64, error) {
			enq = append(enq, p)
			return 1, nil
		})

	res, err := newScheduler(e).RunOnce(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, res.NearNotices)
	require.Len(t, enq, 2)
	assert.Equal(t, domain.KindQueueNear, enq[0].Kind)
	assert.Equal(t, patient, enq[0].UserID)
	var n domain.QueueNearNotice
	require.NoError(t, json.Unmarshal(enq[0].Payload, &n))
	assert.Equal(t, rows[0].ID.String(), n.QueueEntryID)
	assert.Equal(t, 12, n.TicketNo)
	assert.Equal(t, 3, n.AheadCount)
	assert.Equal(t, "内科", n.Department)
}

func TestScheduler_NearThresholdIsConfigurable(t *testing.T) {
	e := newClinicEnv(t)
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, db.ListQueueEntriesNeedingNearNoticeParams{ServiceDate: today(), MaxAhead: 5}).Return(nil, nil)

	s := service.NewReminderScheduler(e.repos, service.WithClinicClock(e.now), service.WithNearThreshold(5), service.WithSchedulerLogger(zerolog.Nop()))
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
}

// 片方が失敗してももう片方は実行する。エラーはまとめて返す
func TestScheduler_RemindersFailureDoesNotSkipNearNotices(t *testing.T) {
	e := newClinicEnv(t)
	*e.clock = day(12, 18, 0)
	boom := errors.New("db down")
	e.appts.EXPECT().ListAppointmentsNeedingReminder(mock.Anything, mock.Anything).Return(nil, boom)
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, mock.Anything).Return([]db.ListQueueEntriesNeedingNearNoticeRow{nearRow(1, 5)}, nil)
	e.notif.EXPECT().EnqueueNotificationOnce(mock.Anything, mock.Anything).Return(int64(1), nil)

	res, err := newScheduler(e).RunOnce(context.Background())
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, 1, res.NearNotices)
}

func TestScheduler_RunLoopStopsOnCancel(t *testing.T) {
	e := newClinicEnv(t)
	e.queue.EXPECT().ListQueueEntriesNeedingNearNotice(mock.Anything, mock.Anything).Return(nil, errors.New("transient"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { newScheduler(e).Run(ctx, 10*time.Millisecond); close(done) }()

	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx のキャンセルで停止しない")
	}
}
