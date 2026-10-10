package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 11-clinic-reservation-slot-api-admin-settings TASK-0001 / REQ-006

type fakeGenerator struct {
	calls int
	err   error
}

func (f *fakeGenerator) GenerateUpcoming(context.Context) (int, error) { f.calls++; return 0, f.err }

func newAdminService(e *clinicEnv, gen *fakeGenerator) service.AdminService {
	return service.NewAdminService(e.repos, e.tx, gen, service.WithClinicClock(e.now))
}

func sched(weekday int, start, end string, slotMin, capacity int) service.ScheduleInput {
	return service.ScheduleInput{Weekday: weekday, StartTime: start, EndTime: end, SlotMinutes: slotMin, Capacity: capacity}
}

func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	var ve *service.ValidationError
	require.ErrorAs(t, err, &ve)
	var out []string
	for _, f := range ve.Fields {
		out = append(out, f.Field)
	}
	return out
}

// ---------- PutSchedules (REQ-006-AC-1) ----------

func TestPutSchedules_ReplacesAndGenerates(t *testing.T) {
	e := newClinicEnv(t)
	gen := &fakeGenerator{}
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(7)).Return(db.Doctor{ID: 7, DepartmentID: 1}, nil)
	e.admin.EXPECT().DeleteDoctorSchedules(mock.Anything, int64(7)).Return(nil).Once()
	var created []db.CreateDoctorScheduleParams
	e.admin.EXPECT().CreateDoctorSchedule(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.CreateDoctorScheduleParams) error {
			created = append(created, p)
			return nil
		})

	err := newAdminService(e, gen).PutSchedules(context.Background(), service.PutSchedulesInput{
		DoctorID:  7,
		Schedules: []service.ScheduleInput{sched(1, "09:00", "12:00", 30, 2), sched(1, "14:00", "17:00", 30, 2), sched(3, "09:00", "12:00", 20, 1)},
	})
	require.NoError(t, err)

	require.Len(t, created, 3)
	assert.EqualValues(t, 1, created[0].Weekday)
	assert.Equal(t, clockOf(9, 0), created[0].StartTime)
	assert.Equal(t, clockOf(12, 0), created[0].EndTime)
	assert.EqualValues(t, 30, created[0].SlotMinutes)
	assert.EqualValues(t, 2, created[0].Capacity)
	assert.Equal(t, 1, gen.calls, "保存後に枠を生成する")
}

// 空のスケジュールは「診療なし」に置き換える（全削除）
func TestPutSchedules_EmptyClearsSchedules(t *testing.T) {
	e := newClinicEnv(t)
	gen := &fakeGenerator{}
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(7)).Return(db.Doctor{ID: 7}, nil)
	e.admin.EXPECT().DeleteDoctorSchedules(mock.Anything, int64(7)).Return(nil)

	require.NoError(t, newAdminService(e, gen).PutSchedules(context.Background(), service.PutSchedulesInput{DoctorID: 7}))
}

func TestPutSchedules_ValidationErrors(t *testing.T) {
	cases := map[string]struct {
		in    []service.ScheduleInput
		field string
	}{
		"終了が開始以前":     {[]service.ScheduleInput{sched(1, "12:00", "09:00", 30, 1)}, "schedules[0].end_time"},
		"開始と終了が同じ":    {[]service.ScheduleInput{sched(1, "09:00", "09:00", 30, 1)}, "schedules[0].end_time"},
		"枠長が診療時間より長い": {[]service.ScheduleInput{sched(1, "09:00", "09:20", 30, 1)}, "schedules[0].slot_minutes"},
		"曜日が範囲外":      {[]service.ScheduleInput{sched(7, "09:00", "12:00", 30, 1)}, "schedules[0].weekday"},
		"曜日が負":        {[]service.ScheduleInput{sched(-1, "09:00", "12:00", 30, 1)}, "schedules[0].weekday"},
		"開始時刻の書式":     {[]service.ScheduleInput{sched(1, "9:00", "12:00", 30, 1)}, "schedules[0].start_time"},
		"終了時刻が24:00":  {[]service.ScheduleInput{sched(1, "09:00", "24:00", 30, 1)}, "schedules[0].end_time"},
		"枠長 0":        {[]service.ScheduleInput{sched(1, "09:00", "12:00", 0, 1)}, "schedules[0].slot_minutes"},
		"定員 0":        {[]service.ScheduleInput{sched(1, "09:00", "12:00", 30, 0)}, "schedules[0].capacity"},
		"同じ曜日で重複":     {[]service.ScheduleInput{sched(1, "09:00", "12:00", 30, 1), sched(1, "11:00", "14:00", 30, 1)}, "schedules[1]"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t)
			gen := &fakeGenerator{}
			e.admin.EXPECT().GetDoctor(mock.Anything, int64(7)).Return(db.Doctor{ID: 7}, nil)
			// 検証エラーのときは DeleteDoctorSchedules / Create が呼ばれない（既存スケジュールを壊さない）

			err := newAdminService(e, gen).PutSchedules(context.Background(), service.PutSchedulesInput{DoctorID: 7, Schedules: c.in})

			assert.Contains(t, fieldsOf(t, err), c.field)
			assert.Zero(t, gen.calls)
		})
	}
}

// 境界: 隣接する時間帯は重複ではない／曜日が違えば同じ時間でも可
func TestPutSchedules_AdjacentAndDifferentWeekdaysAreNotOverlap(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDoctor(mock.Anything, mock.Anything).Return(db.Doctor{ID: 7}, nil)
	e.admin.EXPECT().DeleteDoctorSchedules(mock.Anything, mock.Anything).Return(nil)
	e.admin.EXPECT().CreateDoctorSchedule(mock.Anything, mock.Anything).Return(nil).Times(3)

	err := newAdminService(e, &fakeGenerator{}).PutSchedules(context.Background(), service.PutSchedulesInput{
		DoctorID:  7,
		Schedules: []service.ScheduleInput{sched(1, "09:00", "12:00", 30, 1), sched(1, "12:00", "15:00", 30, 1), sched(2, "09:00", "12:00", 30, 1)},
	})
	assert.NoError(t, err)
}

func TestPutSchedules_UnknownDoctor(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(99)).Return(db.Doctor{}, pgx.ErrNoRows)

	err := newAdminService(e, &fakeGenerator{}).PutSchedules(context.Background(), service.PutSchedulesInput{DoctorID: 99})
	assert.Equal(t, []string{"doctor_id"}, fieldsOf(t, err))
}

// 枠生成に失敗しても保存したスケジュールは有効（ジョブが次回補完する）。エラーは返して運用者に知らせる
func TestPutSchedules_GenerationFailureIsReported(t *testing.T) {
	e := newClinicEnv(t)
	boom := errors.New("db down")
	e.admin.EXPECT().GetDoctor(mock.Anything, mock.Anything).Return(db.Doctor{ID: 7}, nil)
	e.admin.EXPECT().DeleteDoctorSchedules(mock.Anything, mock.Anything).Return(nil)

	err := newAdminService(e, &fakeGenerator{err: boom}).PutSchedules(context.Background(), service.PutSchedulesInput{DoctorID: 7})
	assert.ErrorIs(t, err, boom)
}

// ---------- PutSettings (REQ-006-AC-2) ----------

func ptr[T any](v T) *T { return &v }

func TestPutSettings_PartialUpdateKeepsOtherValue(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: 30}, nil)
	var got db.UpdateClinicSettingsParams
	e.admin.EXPECT().UpdateClinicSettings(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.UpdateClinicSettingsParams) (db.ClinicSetting, error) {
			got = p
			return db.ClinicSetting{ID: 1, CancelDeadlineMinutes: p.CancelDeadlineMinutes, BookingWindowDays: p.BookingWindowDays}, nil
		})

	out, err := newAdminService(e, &fakeGenerator{}).PutSettings(context.Background(), service.SettingsInput{CancelDeadlineMinutes: ptr(120)})
	require.NoError(t, err)

	assert.EqualValues(t, 120, got.CancelDeadlineMinutes)
	assert.EqualValues(t, 30, got.BookingWindowDays, "未指定の項目は現在値を維持")
	assert.Equal(t, 120, out.CancelDeadlineMinutes)
	assert.Equal(t, 30, out.BookingWindowDays)
}

func TestPutSettings_DepartmentSettings(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: 30}, nil)
	e.admin.EXPECT().UpdateClinicSettings(mock.Anything, mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: 30}, nil)
	var got db.UpdateDepartmentSettingsParams
	e.admin.EXPECT().UpdateDepartmentSettings(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.UpdateDepartmentSettingsParams) (db.Department, error) {
			got = p
			return db.Department{ID: p.ID}, nil
		})

	_, err := newAdminService(e, &fakeGenerator{}).PutSettings(context.Background(), service.SettingsInput{
		Departments: []service.DepartmentSettingsInput{{ID: 1, StandardConsultMinutes: ptr(15), WalkinCutoff: ptr("11:30"), WalkinCapacity: ptr(20)}},
	})
	require.NoError(t, err)

	assert.EqualValues(t, 1, got.ID)
	assert.Equal(t, pgtype.Int4{Int32: 15, Valid: true}, got.StandardConsultMinutes)
	assert.Equal(t, clockOf(11, 30), got.WalkinCutoff)
	assert.Equal(t, pgtype.Int4{Int32: 20, Valid: true}, got.WalkinCapacity)
}

// 診療科の項目を省略すると未設定(NULL)に戻る
func TestPutSettings_OmittedDepartmentFieldsBecomeNull(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: 30}, nil)
	e.admin.EXPECT().UpdateClinicSettings(mock.Anything, mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: 30}, nil)
	var got db.UpdateDepartmentSettingsParams
	e.admin.EXPECT().UpdateDepartmentSettings(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.UpdateDepartmentSettingsParams) (db.Department, error) {
			got = p
			return db.Department{}, nil
		})

	_, err := newAdminService(e, &fakeGenerator{}).PutSettings(context.Background(), service.SettingsInput{Departments: []service.DepartmentSettingsInput{{ID: 1}}})
	require.NoError(t, err)

	assert.False(t, got.StandardConsultMinutes.Valid)
	assert.False(t, got.WalkinCutoff.Valid)
	assert.False(t, got.WalkinCapacity.Valid)
}

func TestPutSettings_ValidationErrors(t *testing.T) {
	cases := map[string]struct {
		in    service.SettingsInput
		field string
	}{
		"キャンセル期限が負":    {service.SettingsInput{CancelDeadlineMinutes: ptr(-1)}, "cancel_deadline_minutes"},
		"キャンセル期限が長すぎる": {service.SettingsInput{CancelDeadlineMinutes: ptr(60*24*31 + 1)}, "cancel_deadline_minutes"},
		"受付期間 0":       {service.SettingsInput{BookingWindowDays: ptr(0)}, "booking_window_days"},
		"受付期間が長すぎる":    {service.SettingsInput{BookingWindowDays: ptr(366)}, "booking_window_days"},
		"標準診察時間 0":     {service.SettingsInput{Departments: []service.DepartmentSettingsInput{{ID: 1, StandardConsultMinutes: ptr(0)}}}, "departments[0].standard_consult_minutes"},
		"当日受付締切の書式":    {service.SettingsInput{Departments: []service.DepartmentSettingsInput{{ID: 1, WalkinCutoff: ptr("25:00")}}}, "departments[0].walkin_cutoff"},
		"当日受付定員 0":     {service.SettingsInput{Departments: []service.DepartmentSettingsInput{{ID: 1, WalkinCapacity: ptr(0)}}}, "departments[0].walkin_capacity"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t) // 検証エラーのとき repository は一切呼ばれない
			_, err := newAdminService(e, &fakeGenerator{}).PutSettings(context.Background(), c.in)
			assert.Contains(t, fieldsOf(t, err), c.field)
		})
	}
}

// 境界値: 許容範囲の端は受け付ける
func TestPutSettings_AcceptsBoundaryValues(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1}, nil)
	e.admin.EXPECT().UpdateClinicSettings(mock.Anything, mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 0, BookingWindowDays: 365}, nil)

	out, err := newAdminService(e, &fakeGenerator{}).PutSettings(context.Background(), service.SettingsInput{CancelDeadlineMinutes: ptr(0), BookingWindowDays: ptr(365)})
	require.NoError(t, err)
	assert.Equal(t, 365, out.BookingWindowDays)
}

func TestPutSettings_UnknownDepartment(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: 30}, nil)
	e.admin.EXPECT().UpdateClinicSettings(mock.Anything, mock.Anything).Return(db.ClinicSetting{ID: 1}, nil)
	e.admin.EXPECT().UpdateDepartmentSettings(mock.Anything, mock.Anything).Return(db.Department{}, pgx.ErrNoRows)

	_, err := newAdminService(e, &fakeGenerator{}).PutSettings(context.Background(), service.SettingsInput{Departments: []service.DepartmentSettingsInput{{ID: 99}}})
	assert.Contains(t, fieldsOf(t, err), "departments[0].id")
}

// ---------- 現在の設定・スケジュールの参照（管理者画面用） ----------

func TestGetSettings(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 90, BookingWindowDays: 21}, nil)
	e.admin.EXPECT().ListDepartments(mock.Anything).Return([]db.Department{
		{ID: 1, Name: "内科", StandardConsultMinutes: pgtype.Int4{Int32: 15, Valid: true}, WalkinCutoff: clockOf(11, 30), WalkinCapacity: pgtype.Int4{Int32: 20, Valid: true}},
		{ID: 2, Name: "小児科"},
	}, nil)

	got, err := newAdminService(e, &fakeGenerator{}).GetSettings(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 90, got.CancelDeadlineMinutes)
	assert.Equal(t, 21, got.BookingWindowDays)
	require.Len(t, got.Departments, 2)
	assert.Equal(t, "内科", got.Departments[0].Name)
	assert.Equal(t, 15, *got.Departments[0].StandardConsultMinutes)
	assert.Equal(t, "11:30", *got.Departments[0].WalkinCutoff)
	assert.Equal(t, 20, *got.Departments[0].WalkinCapacity)
	assert.Nil(t, got.Departments[1].StandardConsultMinutes, "未設定は nil")
	assert.Nil(t, got.Departments[1].WalkinCutoff)
	assert.Nil(t, got.Departments[1].WalkinCapacity)
}

func TestGetSchedules(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(7)).Return(db.Doctor{ID: 7}, nil)
	e.admin.EXPECT().ListDoctorSchedules(mock.Anything, int64(7)).Return([]db.DoctorSchedule{
		{DoctorID: 7, Weekday: 1, StartTime: clockOf(9, 0), EndTime: clockOf(12, 0), SlotMinutes: 30, Capacity: 2},
		{DoctorID: 7, Weekday: 3, StartTime: clockOf(14, 5), EndTime: clockOf(17, 0), SlotMinutes: 20, Capacity: 1},
	}, nil)

	got, err := newAdminService(e, &fakeGenerator{}).GetSchedules(context.Background(), 7)
	require.NoError(t, err)

	assert.Equal(t, []service.ScheduleInput{
		{Weekday: 1, StartTime: "09:00", EndTime: "12:00", SlotMinutes: 30, Capacity: 2},
		{Weekday: 3, StartTime: "14:05", EndTime: "17:00", SlotMinutes: 20, Capacity: 1},
	}, got)
}

func TestGetSchedules_EmptyAndUnknownDoctor(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDoctor(mock.Anything, int64(7)).Return(db.Doctor{ID: 7}, nil)
	e.admin.EXPECT().ListDoctorSchedules(mock.Anything, int64(7)).Return(nil, nil)
	got, err := newAdminService(e, &fakeGenerator{}).GetSchedules(context.Background(), 7)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)

	e2 := newClinicEnv(t)
	e2.admin.EXPECT().GetDoctor(mock.Anything, int64(99)).Return(db.Doctor{}, pgx.ErrNoRows)
	_, err = newAdminService(e2, &fakeGenerator{}).GetSchedules(context.Background(), 99)
	assert.Equal(t, []string{"doctor_id"}, fieldsOf(t, err))
}
