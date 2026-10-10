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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 16-clinic-reservation-closures TASK-0001 / REQ-005-AC-3

var staffID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

func newClosureService(e *clinicEnv) service.ClosureService {
	return service.NewClosureService(e.repos, e.tx, service.WithClinicClock(e.now))
}

func doctorOf(id int64) *int64 { return &id }

func noticeRow(no string, slot uuid.UUID, start time.Time) db.ListBookedAppointmentsForNoticeRow {
	return db.ListBookedAppointmentsForNoticeRow{ID: uuid.New(), AppointmentNo: no, PatientID: patient, SlotID: slot, StartAt: tsOf(start), DepartmentName: "内科", DoctorName: "山田"}
}

func TestClosure_Success(t *testing.T) {
	e := newClinicEnv(t)
	from, to := day(13, 9, 0), day(13, 12, 0)
	s1, s2, s3 := uuid.New(), uuid.New(), uuid.New()
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(7)).Return(db.Doctor{ID: 7}, nil)
	var order []string
	e.admin.EXPECT().CreateClosure(mock.Anything, db.CreateClosureParams{DoctorID: pgtype.Int8{Int64: 7, Valid: true}, FromAt: tsOf(from), ToAt: tsOf(to), Reason: "学会"}).
		RunAndReturn(func(context.Context, db.CreateClosureParams) error { order = append(order, "closure"); return nil })
	e.slots.EXPECT().CloseSlots(mock.Anything, db.CloseSlotsParams{DoctorID: pgtype.Int8{Int64: 7, Valid: true}, FromAt: tsOf(from), ToAt: tsOf(to)}).
		RunAndReturn(func(context.Context, db.CloseSlotsParams) ([]uuid.UUID, error) {
			order = append(order, "close-slots")
			return []uuid.UUID{s1, s2}, nil // s3 は既に closed だった枠（今回は閉じていない）
		})
	e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, db.ListBookedAppointmentsForNoticeParams{DoctorID: pgtype.Int8{Int64: 7, Valid: true}, FromAt: tsOf(from), ToAt: tsOf(to)}).
		Return([]db.ListBookedAppointmentsForNoticeRow{noticeRow("A-1", s1, day(13, 9, 30)), noticeRow("A-2", s2, day(13, 10, 0)), noticeRow("A-3", s3, day(13, 11, 0))}, nil)
	var notices []db.EnqueueNotificationParams
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationParams) (db.Notification, error) {
			notices = append(notices, p)
			return db.Notification{}, nil
		})
	var audit db.InsertAuditLogParams
	e.admin.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.InsertAuditLogParams) error { audit = p; return nil })

	res, err := newClosureService(e).Create(context.Background(), staffID, service.ClosureInput{DoctorID: doctorOf(7), From: from, To: to, Reason: "学会"})
	require.NoError(t, err)

	assert.Equal(t, 2, res.ClosedSlots)
	assert.Equal(t, 2, res.Notified, "今回新たに閉じた枠の予約者だけに通知（既に休診済みの枠は通知済み）")
	assert.Equal(t, []string{"closure", "close-slots"}, order)
	require.Len(t, notices, 2)
	assert.Equal(t, domain.KindClinicClosure, notices[0].Kind)
	assert.Equal(t, patient, notices[0].UserID)
	var n domain.AppointmentNotice
	require.NoError(t, json.Unmarshal(notices[0].Payload, &n))
	assert.Equal(t, "A-1", n.AppointmentNo)
	assert.Equal(t, "内科", n.Department)
	assert.True(t, n.StartAt.Equal(day(13, 9, 30)))
	assert.Equal(t, pgtype.UUID{Bytes: staffID, Valid: true}, audit.ActorID)
	assert.Equal(t, "create_closure", audit.Action)
	assert.Equal(t, "doctor:7", audit.Target)
}

// 休診の自由記述の理由は通知に含めない（意図しない情報の送信を防ぐ）
func TestClosure_ReasonIsNeverSentToPatients(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(nil)
	s := uuid.New()
	e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).Return([]uuid.UUID{s}, nil)
	e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, mock.Anything).Return([]db.ListBookedAppointmentsForNoticeRow{noticeRow("A-1", s, day(13, 9, 30))}, nil)
	var payload string
	e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.EnqueueNotificationParams) (db.Notification, error) {
			payload = string(p.Payload)
			return db.Notification{}, nil
		})
	e.admin.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).Return(nil)

	_, err := newClosureService(e).Create(context.Background(), staffID, service.ClosureInput{From: day(13, 9, 0), To: day(13, 12, 0), Reason: "院長の入院のため"})
	require.NoError(t, err)
	assert.NotContains(t, payload, "入院")
	assert.NotContains(t, payload, "reason")
}

// 全体休診（医師指定なし）
func TestClosure_WholeClinic(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().CreateClosure(mock.Anything, db.CreateClosureParams{FromAt: tsOf(day(13, 0, 0)), ToAt: tsOf(day(14, 0, 0))}).Return(nil)
	e.slots.EXPECT().CloseSlots(mock.Anything, db.CloseSlotsParams{FromAt: tsOf(day(13, 0, 0)), ToAt: tsOf(day(14, 0, 0))}).Return(nil, nil)
	e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, db.ListBookedAppointmentsForNoticeParams{FromAt: tsOf(day(13, 0, 0)), ToAt: tsOf(day(14, 0, 0))}).Return(nil, nil)
	var audit db.InsertAuditLogParams
	e.admin.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p db.InsertAuditLogParams) error { audit = p; return nil })

	res, err := newClosureService(e).Create(context.Background(), staffID, service.ClosureInput{From: day(13, 0, 0), To: day(14, 0, 0)})
	require.NoError(t, err)
	assert.Zero(t, res.ClosedSlots)
	assert.Zero(t, res.Notified)
	assert.Equal(t, "clinic", audit.Target)
}

func TestClosure_Validation(t *testing.T) {
	cases := map[string]struct {
		in    service.ClosureInput
		field string
	}{
		"開始が終了以降":     {service.ClosureInput{From: day(13, 12, 0), To: day(13, 9, 0)}, "to_at"},
		"開始と終了が同じ":    {service.ClosureInput{From: day(13, 9, 0), To: day(13, 9, 0)}, "to_at"},
		"終了が現在以前（過去）": {service.ClosureInput{From: day(11, 9, 0), To: day(12, 9, 0)}, "to_at"}, // 現在 12 日 9:00
		"期間が長すぎる":     {service.ClosureInput{From: day(13, 9, 0), To: day(13, 9, 0).AddDate(0, 0, 400)}, "to_at"},
		"理由が長すぎる":     {service.ClosureInput{From: day(13, 9, 0), To: day(13, 12, 0), Reason: strings.Repeat("あ", 201)}, "reason"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t) // repository は一切呼ばれない（何も変更しない）
			_, err := newClosureService(e).Create(context.Background(), staffID, c.in)
			var ve *service.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, c.field, ve.Fields[0].Field)
		})
	}
}

// 境界: 開始が過去でも、終了が未来なら（進行中の休診として）設定できる／理由ちょうど 200 文字は可
func TestClosure_AcceptsOngoingAndBoundaryReason(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(nil)
	e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).Return(nil, nil)
	e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, mock.Anything).Return(nil, nil)
	e.admin.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).Return(nil)

	_, err := newClosureService(e).Create(context.Background(), staffID, service.ClosureInput{From: day(12, 8, 0), To: day(12, 9, 1), Reason: strings.Repeat("あ", 200)})
	assert.NoError(t, err)
}

func TestClosure_UnknownDoctor(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(99)).Return(db.Doctor{}, pgx.ErrNoRows)

	_, err := newClosureService(e).Create(context.Background(), staffID, service.ClosureInput{DoctorID: doctorOf(99), From: day(13, 9, 0), To: day(13, 12, 0)})
	var ve *service.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "doctor_id", ve.Fields[0].Field)
}

// どれか 1 つが失敗したら全体が失敗（Tx が巻き戻る）
func TestClosure_AnyStepFailureFailsTheWholeOperation(t *testing.T) {
	boom := errors.New("db down")
	steps := map[string]func(e *clinicEnv){
		"休診の作成": func(e *clinicEnv) { e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(boom) },
		"枠を閉じる": func(e *clinicEnv) {
			e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(nil)
			e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).Return(nil, boom)
		},
		"予約者の取得": func(e *clinicEnv) {
			e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(nil)
			e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).Return(nil, nil)
			e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, mock.Anything).Return(nil, boom)
		},
		"通知の登録": func(e *clinicEnv) {
			s := uuid.New()
			e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(nil)
			e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).Return([]uuid.UUID{s}, nil)
			e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, mock.Anything).Return([]db.ListBookedAppointmentsForNoticeRow{noticeRow("A", s, day(13, 9, 30))}, nil)
			e.notif.EXPECT().EnqueueNotification(mock.Anything, mock.Anything).Return(db.Notification{}, boom)
		},
		"監査ログ": func(e *clinicEnv) {
			e.admin.EXPECT().CreateClosure(mock.Anything, mock.Anything).Return(nil)
			e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).Return(nil, nil)
			e.appts.EXPECT().ListBookedAppointmentsForNotice(mock.Anything, mock.Anything).Return(nil, nil)
			e.admin.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).Return(boom)
		},
	}
	for name, setup := range steps {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t)
			setup(e)
			res, err := newClosureService(e).Create(context.Background(), staffID, service.ClosureInput{From: day(13, 9, 0), To: day(13, 12, 0)})
			assert.ErrorIs(t, err, boom)
			assert.Nil(t, res)
		})
	}
}
