package service_test

import (
	"context"
	"errors"
	"github.com/rs/zerolog"
	"sync"
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

// IMP 11-clinic-reservation-slot-api-admin-settings TASK-0001

func newSlotService(e *clinicEnv) service.SlotService {
	return service.NewSlotService(e.repos, e.tx, service.WithClinicClock(e.now))
}

func settings(window int32) db.ClinicSetting {
	return db.ClinicSetting{ID: 1, CancelDeadlineMinutes: 60, BookingWindowDays: window}
}

func slotAt(t time.Time, capacity, booked int32, status string) db.Slot {
	return db.Slot{ID: uuid.New(), DepartmentID: 1, DoctorID: 1, StartAt: tsOf(t), Capacity: capacity, BookedCount: booked, Status: status}
}

func day(d int, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, domain.JST) }

// ---------- 空き枠検索 (REQ-001-AC-1/3) ----------

func TestListSlots_ReasonsAndOrder(t *testing.T) {
	e := newClinicEnv(t) // 現在 2026-10-12 09:00 JST
	e.admin.EXPECT().GetDepartment(mock.Anything, int64(1)).Return(db.Department{ID: 1}, nil)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(settings(30), nil)
	rows := []db.Slot{
		slotAt(day(12, 8, 30), 2, 0, "open"),    // 過去
		slotAt(day(12, 9, 0), 2, 0, "open"),     // 現在ちょうど = 受付締切後
		slotAt(day(12, 9, 30), 2, 0, "open"),    // 空き
		slotAt(day(12, 10, 0), 2, 2, "open"),    // 満席
		slotAt(day(12, 10, 30), 2, 0, "closed"), // 休診
		slotAt(day(12, 11, 0), 2, 2, "closed"),  // 休診かつ満席 → closed が優先
	}
	e.slots.EXPECT().ListSlots(mock.Anything, mock.Anything).Return(rows, nil)

	got, err := newSlotService(e).ListSlots(context.Background(), service.SlotQuery{DepartmentID: 1, Date: day(12, 0, 0)})
	require.NoError(t, err)

	require.Len(t, got, 6)
	want := []struct {
		avail  bool
		reason string
	}{{false, "past_cutoff"}, {false, "past_cutoff"}, {true, ""}, {false, "full"}, {false, "closed"}, {false, "closed"}}
	for i, w := range want {
		assert.Equal(t, w.avail, got[i].Available, "slot %d", i)
		assert.Equal(t, w.reason, got[i].Reason, "slot %d", i)
	}
	assert.Equal(t, rows[2].ID, got[2].ID)
}

// 受付期間: 当日 + window 日までは可、翌日は outside_window
func TestListSlots_BookingWindowBoundary(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDepartment(mock.Anything, int64(1)).Return(db.Department{ID: 1}, nil)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(settings(7), nil)
	e.slots.EXPECT().ListSlots(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.ListSlotsParams) ([]db.Slot, error) {
			return []db.Slot{slotAt(p.FromAt.Time.Add(10*time.Hour), 1, 0, "open")}, nil
		})
	svc := newSlotService(e)

	last, err := svc.ListSlots(context.Background(), service.SlotQuery{DepartmentID: 1, Date: day(19, 0, 0)}) // 12 日 + 7 日
	require.NoError(t, err)
	assert.True(t, last[0].Available, "受付期間の最終日は予約可")

	beyond, err := svc.ListSlots(context.Background(), service.SlotQuery{DepartmentID: 1, Date: day(20, 0, 0)})
	require.NoError(t, err)
	assert.False(t, beyond[0].Available)
	assert.Equal(t, "outside_window", beyond[0].Reason)
}

// 検索範囲は指定日の JST 0:00〜翌 0:00、医師指名は絞り込みに渡る
func TestListSlots_QueryParams(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDepartment(mock.Anything, int64(3)).Return(db.Department{ID: 3}, nil)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(settings(30), nil)
	var got db.ListSlotsParams
	e.slots.EXPECT().ListSlots(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.ListSlotsParams) ([]db.Slot, error) { got = p; return nil, nil })
	doctor := int64(42)

	out, err := newSlotService(e).ListSlots(context.Background(), service.SlotQuery{DepartmentID: 3, DoctorID: &doctor, Date: day(15, 0, 0)})
	require.NoError(t, err)

	assert.Empty(t, out)
	assert.NotNil(t, out, "空でも null ではなく空配列（JSON で [] になる）")
	assert.Equal(t, int64(3), got.DepartmentID)
	assert.Equal(t, pgtype.Int8{Int64: 42, Valid: true}, got.DoctorID)
	assert.True(t, got.FromAt.Time.Equal(day(15, 0, 0)))
	assert.True(t, got.ToAt.Time.Equal(day(16, 0, 0)))
}

func TestListSlots_UnknownDepartmentIsValidationError(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetDepartment(mock.Anything, int64(99)).Return(db.Department{}, pgx.ErrNoRows)

	_, err := newSlotService(e).ListSlots(context.Background(), service.SlotQuery{DepartmentID: 99, Date: day(12, 0, 0)})

	var ve *service.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "department_id", ve.Fields[0].Field)
}

func TestListSlots_RepositoryErrorsPropagate(t *testing.T) {
	e := newClinicEnv(t)
	boom := errors.New("db down")
	e.admin.EXPECT().GetDepartment(mock.Anything, mock.Anything).Return(db.Department{}, boom)

	_, err := newSlotService(e).ListSlots(context.Background(), service.SlotQuery{DepartmentID: 1, Date: day(12, 0, 0)})
	assert.ErrorIs(t, err, boom)
}

func TestListDepartmentsAndDoctors(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().ListDepartments(mock.Anything).Return([]db.Department{{ID: 1, Name: "内科"}}, nil)
	e.admin.EXPECT().ListDoctors(mock.Anything, int64(1)).Return([]db.Doctor{{ID: 7, DepartmentID: 1, Name: "山田"}}, nil)
	svc := newSlotService(e)

	deps, err := svc.ListDepartments(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "内科", deps[0].Name)
	docs, err := svc.ListDoctors(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "山田", docs[0].Name)
}

// ---------- 枠の生成 (REQ-006-AC-1) ----------

func schedule(doctorID int64, weekday int16, sh, sm, eh, em int, slotMin, capacity int32) db.ListSchedulesByWeekdayRow {
	return db.ListSchedulesByWeekdayRow{DoctorID: doctorID, DepartmentID: 1, Weekday: weekday, StartTime: clockOf(sh, sm), EndTime: clockOf(eh, em), SlotMinutes: slotMin, Capacity: capacity}
}

func collectCreates(e *clinicEnv) *[]db.CreateSlotIfAbsentParams {
	var created []db.CreateSlotIfAbsentParams
	e.slots.EXPECT().CreateSlotIfAbsent(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.CreateSlotIfAbsentParams) error { created = append(created, p); return nil }).Maybe()
	return &created
}

func TestGenerateSlots_MondayMorning(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, int16(1)).Return([]db.ListSchedulesByWeekdayRow{schedule(7, 1, 9, 0, 12, 0, 30, 2)}, nil)
	e.admin.EXPECT().ListClosuresFrom(mock.Anything, mock.Anything).Return(nil, nil)
	created := collectCreates(e)

	n, err := newSlotService(e).GenerateSlots(context.Background(), day(12, 0, 0), day(12, 0, 0)) // 月曜
	require.NoError(t, err)

	assert.Equal(t, 6, n)
	require.Len(t, *created, 6)
	assert.True(t, (*created)[0].StartAt.Time.Equal(day(12, 9, 0)))
	assert.True(t, (*created)[5].StartAt.Time.Equal(day(12, 11, 30)), "終了時刻ちょうど開始の枠は作らない")
	assert.EqualValues(t, 2, (*created)[0].Capacity)
	assert.EqualValues(t, 7, (*created)[0].DoctorID)
	assert.EqualValues(t, 1, (*created)[0].DepartmentID)
}

// 枠長が診療時間を割り切れない場合は、枠が終了時刻に収まるものだけ作る
func TestGenerateSlots_SlotLengthNotDividingSpan(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, int16(1)).Return([]db.ListSchedulesByWeekdayRow{schedule(7, 1, 9, 0, 10, 0, 25, 1)}, nil)
	e.admin.EXPECT().ListClosuresFrom(mock.Anything, mock.Anything).Return(nil, nil)
	created := collectCreates(e)

	_, err := newSlotService(e).GenerateSlots(context.Background(), day(12, 0, 0), day(12, 0, 0))
	require.NoError(t, err)

	require.Len(t, *created, 2, "9:00, 9:25 のみ（9:50 開始は 10:15 に終わり超過）")
}

func TestGenerateSlots_MultipleDaysUseTheirWeekdays(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, int16(1)).Return([]db.ListSchedulesByWeekdayRow{schedule(7, 1, 9, 0, 10, 0, 30, 1)}, nil) // 月
	for _, wd := range []int16{2, 3, 4, 5, 6} {                                                                                                      // 火〜土: 診療なし
		e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, wd).Return(nil, nil)
	}
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, int16(0)).Return([]db.ListSchedulesByWeekdayRow{schedule(8, 0, 10, 0, 11, 0, 30, 1)}, nil) // 日
	e.admin.EXPECT().ListClosuresFrom(mock.Anything, mock.Anything).Return(nil, nil)
	created := collectCreates(e)

	n, err := newSlotService(e).GenerateSlots(context.Background(), day(12, 0, 0), day(18, 0, 0)) // 月〜日
	require.NoError(t, err)
	_ = n

	// 月(12日)に 2 枠、日(18日)に 2 枠。火〜土はスケジュールが無い曜日は呼ばれない設定なので mock が厳密に検証する
	assert.Len(t, *created, 4)
}

// 既存の休診期間は、生成した枠にも再適用する
func TestGenerateSlots_ReappliesExistingClosures(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, int16(1)).Return([]db.ListSchedulesByWeekdayRow{schedule(7, 1, 9, 0, 10, 0, 30, 1)}, nil)
	e.admin.EXPECT().ListClosuresFrom(mock.Anything, mock.Anything).Return([]db.ListClosuresFromRow{
		{DoctorID: pgtype.Int8{Int64: 7, Valid: true}, FromAt: tsOf(day(12, 9, 0)), ToAt: tsOf(day(12, 12, 0))},
		{FromAt: tsOf(day(13, 0, 0)), ToAt: tsOf(day(14, 0, 0))}, // 全体休診
	}, nil)
	collectCreates(e)
	var closed []db.CloseSlotsParams
	e.slots.EXPECT().CloseSlots(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p db.CloseSlotsParams) ([]uuid.UUID, error) {
			closed = append(closed, p)
			return nil, nil
		})

	_, err := newSlotService(e).GenerateSlots(context.Background(), day(12, 0, 0), day(12, 0, 0))
	require.NoError(t, err)

	require.Len(t, closed, 2)
	assert.Equal(t, int64(7), closed[0].DoctorID.Int64)
	assert.False(t, closed[1].DoctorID.Valid, "全体休診は doctor_id 指定なし")
}

func TestGenerateSlots_RejectsInvalidRange(t *testing.T) {
	e := newClinicEnv(t)
	svc := newSlotService(e)

	_, err := svc.GenerateSlots(context.Background(), day(13, 0, 0), day(12, 0, 0))
	assert.Error(t, err, "to < from")
	_, err = svc.GenerateSlots(context.Background(), day(1, 0, 0), day(1, 0, 0).AddDate(0, 0, 400))
	assert.Error(t, err, "範囲が長すぎる（誤操作で大量生成しない）")
}

func TestGenerateSlots_StopsOnCreateError(t *testing.T) {
	e := newClinicEnv(t)
	boom := errors.New("db down")
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, mock.Anything).Return([]db.ListSchedulesByWeekdayRow{schedule(7, 1, 9, 0, 10, 0, 30, 1)}, nil)
	e.slots.EXPECT().CreateSlotIfAbsent(mock.Anything, mock.Anything).Return(boom)

	_, err := newSlotService(e).GenerateSlots(context.Background(), day(12, 0, 0), day(12, 0, 0))
	assert.ErrorIs(t, err, boom)
}

// 当日から受付期間の最終日まで生成する
func TestGenerateUpcoming_CoversBookingWindow(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().GetClinicSettings(mock.Anything).Return(settings(2), nil)
	e.admin.EXPECT().ListSchedulesByWeekday(mock.Anything, mock.Anything).Return(nil, nil).Times(3) // 12, 13, 14 日
	e.admin.EXPECT().ListClosuresFrom(mock.Anything, mock.Anything).Return(nil, nil)

	_, err := newSlotService(e).GenerateUpcoming(context.Background())
	assert.NoError(t, err)
}

// ---------- 定期実行 ----------

type countingGen struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (c *countingGen) GenerateUpcoming(context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return 1, c.err
}
func (c *countingGen) count() int { c.mu.Lock(); defer c.mu.Unlock(); return c.calls }

func TestRunSlotGenerator_RunsImmediatelyThenPeriodicallyAndStops(t *testing.T) {
	gen := &countingGen{err: errors.New("transient")} // 失敗しても継続する
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { service.RunSlotGenerator(ctx, gen, 10*time.Millisecond, zerolog.Nop()); close(done) }()

	require.Eventually(t, func() bool { return gen.count() >= 3 }, 2*time.Second, 5*time.Millisecond, "即時 1 回 + 定期実行")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx のキャンセルで停止しない")
	}
}
