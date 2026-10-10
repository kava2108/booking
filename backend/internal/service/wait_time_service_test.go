package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 15-clinic-reservation-wait-time-api TASK-0001 / REQ-004

func newWaitService(e *clinicEnv) service.WaitTimeService {
	return service.NewWaitTimeService(e.repos, service.WithClinicClock(e.now))
}

func std(minutes int32) db.Department {
	return db.Department{ID: 1, Name: "内科", StandardConsultMinutes: pgtype.Int4{Int32: minutes, Valid: true}}
}

// scenario は 1 診療科の算出に必要な repository の応答をまとめて設定する。
type scenario struct {
	dept      db.Department
	onDuty    int64
	scheduled int64 // onDuty==0 のときだけ参照される
	later     int64
	waiting   int64
	samples   []float64
}

func (e *clinicEnv) expect(sc scenario) {
	e.admin.EXPECT().ListDepartments(mock.Anything).Return([]db.Department{sc.dept}, nil).Maybe()
	e.queue.EXPECT().CountWaiting(mock.Anything, db.CountWaitingParams{DepartmentID: sc.dept.ID, ServiceDate: today()}).Return(sc.waiting, nil).Maybe()
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, mock.Anything).Return(sc.onDuty, nil).Maybe()
	e.queue.EXPECT().CountScheduledDoctors(mock.Anything, mock.Anything).Return(sc.scheduled, nil).Maybe()
	e.queue.EXPECT().CountDoctorsStartingLater(mock.Anything, mock.Anything).Return(sc.later, nil).Maybe()
	e.queue.EXPECT().RecentConsultSeconds(mock.Anything, db.RecentConsultSecondsParams{DepartmentID: sc.dept.ID, ServiceDate: today(), Limit: 20}).Return(sc.samples, nil).Maybe()
}

func get(t *testing.T, e *clinicEnv) service.DepartmentWait {
	t.Helper()
	res, err := newWaitService(e).Get(context.Background())
	require.NoError(t, err)
	require.Len(t, res.Departments, 1)
	return res.Departments[0]
}

func secs(minutes ...float64) []float64 {
	out := make([]float64, len(minutes))
	for i, m := range minutes {
		out[i] = m * 60
	}
	return out
}

// ---------- 算出式 (REQ-004-AC-2) ----------

// 待ち時間 = ceil(待ち人数 × 平均所要時間 ÷ 診察中の医師数)
func TestWait_Formula(t *testing.T) {
	cases := map[string]struct {
		waiting int64
		lanes   int64
		samples []float64
		want    int
	}{
		"6 人 × 10 分 ÷ 2 医師 = 30 分":      {6, 2, secs(10, 10, 10), 30},
		"7 人 × 10 分 ÷ 2 医師 = 35 分":      {7, 2, secs(10, 10, 10), 35},
		"切り上げ: 1 人 × 10 分 ÷ 3 医師 = 4 分": {1, 3, secs(10, 10, 10), 4},
		"平均は実績の平均 (10,20,30 → 20)":      {3, 1, secs(10, 20, 30), 60},
		"端数の平均 (10,10,11 分 → 10.33 分)":  {3, 1, secs(10, 10, 11), 31},
		"待ち 0 人は 0 分":                   {0, 2, secs(10, 10, 10), 0},
		"医師 1 人":                        {4, 1, secs(15, 15, 15), 60},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t)
			e.expect(scenario{dept: db.Department{ID: 1, Name: "内科"}, onDuty: c.lanes, waiting: c.waiting, samples: c.samples})

			w := get(t, e)
			assert.Equal(t, "available", w.Status)
			assert.Equal(t, int(c.waiting), w.WaitingCount)
			assert.Equal(t, c.want, w.WaitMinutes)
		})
	}
}

// 実績 3 件以上は actual、未満は管理者設定の標準診察時間 (standard)
func TestWait_BasisSwitchesAtThreeSamples(t *testing.T) {
	t.Run("3 件 → actual（標準時間があっても実績を使う）", func(t *testing.T) {
		e := newClinicEnv(t)
		e.expect(scenario{dept: std(30), onDuty: 1, waiting: 2, samples: secs(10, 10, 10)})
		w := get(t, e)
		assert.Equal(t, "actual", w.Basis)
		assert.Equal(t, 20, w.WaitMinutes, "標準 30 分ではなく実績 10 分で計算")
	})
	t.Run("2 件 → standard", func(t *testing.T) {
		e := newClinicEnv(t)
		e.expect(scenario{dept: std(30), onDuty: 1, waiting: 2, samples: secs(10, 10)})
		w := get(t, e)
		assert.Equal(t, "standard", w.Basis)
		assert.Equal(t, 60, w.WaitMinutes, "少ない実績は使わず標準 30 分で計算")
	})
	t.Run("0 件 → standard", func(t *testing.T) {
		e := newClinicEnv(t)
		e.expect(scenario{dept: std(15), onDuty: 2, waiting: 4})
		w := get(t, e)
		assert.Equal(t, "standard", w.Basis)
		assert.Equal(t, 30, w.WaitMinutes)
	})
}

// REQ-004-AC-5: 実績不足かつ標準時間も無ければ推測せず「算出不可」
func TestWait_NoDataIsUnavailableNotGuessed(t *testing.T) {
	e := newClinicEnv(t)
	e.expect(scenario{dept: db.Department{ID: 1, Name: "内科"}, onDuty: 1, waiting: 5, samples: secs(10, 10)})

	w := get(t, e)
	assert.Equal(t, "unavailable", w.Status)
	assert.Equal(t, "no_data", w.Reason)
	assert.Zero(t, w.WaitMinutes)
	assert.Empty(t, w.Basis)
}

// 待っている人がいなければ、データが無くても 0 分（事実）
func TestWait_NobodyWaitingIsZeroEvenWithoutData(t *testing.T) {
	e := newClinicEnv(t)
	e.expect(scenario{dept: db.Department{ID: 1, Name: "内科"}, onDuty: 1, waiting: 0})

	w := get(t, e)
	assert.Equal(t, "available", w.Status)
	assert.Zero(t, w.WaitMinutes)
	assert.Zero(t, w.WaitingCount)
	assert.Empty(t, w.Basis, "根拠となるデータが無いので basis は付けない")
}

// 診察の記録漏れ（完了し忘れ）による極端に長い所要時間が平均を歪めない
func TestWait_OutlierSamplesAreCapped(t *testing.T) {
	e := newClinicEnv(t)
	e.expect(scenario{dept: std(10), onDuty: 1, waiting: 1, samples: []float64{600, 600, 20 * 3600}}) // 10 分, 10 分, 20 時間

	w := get(t, e)
	// 上限 3 時間: (10 + 10 + 180) / 3 = 66.67 分 → 67 分。上限が無ければ約 407 分になる
	assert.Equal(t, 67, w.WaitMinutes)
}

// ---------- 算出不可の理由 (REQ-004-AC-5) ----------

func TestWait_UnavailableReasons(t *testing.T) {
	cases := map[string]struct {
		sc     scenario
		reason string
	}{
		"臨時休診中（診療時間内だが休診）": {scenario{dept: std(10), onDuty: 0, scheduled: 1, waiting: 3}, "closed"},
		"診療開始前":        {scenario{dept: std(10), onDuty: 0, scheduled: 0, later: 1, waiting: 0}, "before_hours"},
		"昼休み（午後に診療あり）": {scenario{dept: std(10), onDuty: 0, scheduled: 0, later: 1, waiting: 2}, "before_hours"},
		"診療終了後・休診日":    {scenario{dept: std(10), onDuty: 0, scheduled: 0, later: 0, waiting: 0}, "closed"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newClinicEnv(t)
			c.sc.dept.ID = 1
			e.expect(c.sc)

			w := get(t, e)
			assert.Equal(t, "unavailable", w.Status)
			assert.Equal(t, c.reason, w.Reason)
			assert.Zero(t, w.WaitMinutes)
			assert.Empty(t, w.Basis)
		})
	}
}

func TestWait_MultipleDepartments(t *testing.T) {
	e := newClinicEnv(t)
	e.admin.EXPECT().ListDepartments(mock.Anything).Return([]db.Department{{ID: 1, Name: "内科", StandardConsultMinutes: pgtype.Int4{Int32: 10, Valid: true}}, {ID: 2, Name: "小児科"}}, nil)
	// 算出不可の診療科(2)では待ち人数を数えない
	e.queue.EXPECT().CountWaiting(mock.Anything, db.CountWaitingParams{DepartmentID: 1, ServiceDate: today()}).Return(int64(2), nil)
	e.queue.EXPECT().RecentConsultSeconds(mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, db.CountOnDutyDoctorsParams{DepartmentID: 1, At: tsOf(clinicNow)}).Return(int64(1), nil)
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, db.CountOnDutyDoctorsParams{DepartmentID: 2, At: tsOf(clinicNow)}).Return(int64(0), nil)
	e.queue.EXPECT().CountScheduledDoctors(mock.Anything, mock.Anything).Return(int64(0), nil)
	e.queue.EXPECT().CountDoctorsStartingLater(mock.Anything, mock.Anything).Return(int64(1), nil)

	res, err := newWaitService(e).Get(context.Background())
	require.NoError(t, err)

	require.Len(t, res.Departments, 2)
	assert.Equal(t, "available", res.Departments[0].Status)
	assert.Equal(t, 20, res.Departments[0].WaitMinutes)
	assert.Equal(t, "unavailable", res.Departments[1].Status, "診療科ごとに独立して判定")
	assert.True(t, res.UpdatedAt.Equal(clinicNow))
}

func TestWait_RepositoryErrorsPropagate(t *testing.T) {
	e := newClinicEnv(t)
	boom := errors.New("db down")
	e.admin.EXPECT().ListDepartments(mock.Anything).Return(nil, boom)

	_, err := newWaitService(e).Get(context.Background())
	assert.ErrorIs(t, err, boom)
}

// ---------- キャッシュ (10 秒 TTL) ----------

func TestWait_CacheTTL(t *testing.T) {
	e := newClinicEnv(t)
	calls := 0
	e.admin.EXPECT().ListDepartments(mock.Anything).RunAndReturn(func(context.Context) ([]db.Department, error) {
		calls++
		return []db.Department{std(10)}, nil
	})
	e.queue.EXPECT().CountWaiting(mock.Anything, mock.Anything).Return(int64(1), nil)
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, mock.Anything).Return(int64(1), nil)
	e.queue.EXPECT().RecentConsultSeconds(mock.Anything, mock.Anything).Return(nil, nil)
	svc := newWaitService(e)

	first, err := svc.Get(context.Background())
	require.NoError(t, err)
	*e.clock = clinicNow.Add(9*time.Second + 999*time.Millisecond)
	second, err := svc.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "TTL 内は再計算しない")
	assert.True(t, second.UpdatedAt.Equal(first.UpdatedAt), "updated_at は算出時刻のまま")

	*e.clock = clinicNow.Add(10 * time.Second)
	third, err := svc.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "TTL ちょうどで再計算")
	assert.True(t, third.UpdatedAt.Equal(*e.clock))
}

func TestWait_ErrorsAreNotCached(t *testing.T) {
	e := newClinicEnv(t)
	n := 0
	e.admin.EXPECT().ListDepartments(mock.Anything).RunAndReturn(func(context.Context) ([]db.Department, error) {
		n++
		if n == 1 {
			return nil, errors.New("transient")
		}
		return nil, nil
	})
	svc := newWaitService(e)

	_, err := svc.Get(context.Background())
	require.Error(t, err)
	_, err = svc.Get(context.Background())
	assert.NoError(t, err, "失敗の直後でも次の呼び出しで再計算する")
}

// 同時アクセスが集中しても、算出（DB アクセス）は 1 回
func TestWait_ConcurrentRequestsComputeOnce(t *testing.T) {
	e := newClinicEnv(t)
	var mu sync.Mutex
	calls := 0
	e.admin.EXPECT().ListDepartments(mock.Anything).RunAndReturn(func(context.Context) ([]db.Department, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return []db.Department{std(10)}, nil
	})
	e.queue.EXPECT().CountWaiting(mock.Anything, mock.Anything).Return(int64(1), nil)
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, mock.Anything).Return(int64(1), nil)
	e.queue.EXPECT().RecentConsultSeconds(mock.Anything, mock.Anything).Return(nil, nil)
	svc := newWaitService(e)

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Get(context.Background())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, calls)
}

// ---------- 本人の順番 (REQ-004-AC-4) ----------

func entryFor(status string, sort time.Time) db.QueueEntry {
	return db.QueueEntry{ID: entryID, DepartmentID: 1, AppointmentID: puuid(apptID), PatientID: puuid(patient), Status: status, ServiceDate: today(), SortTime: tsOf(sort)}
}

func expectOwnAppointment(e *clinicEnv) {
	e.appts.EXPECT().GetAppointment(mock.Anything, db.GetAppointmentParams{ID: apptID, PatientID: patient}).Return(booked(apptID, slotID, patient), nil)
}

func TestPosition_WaitingShowsAheadAndEstimatedCall(t *testing.T) {
	e := newClinicEnv(t) // 現在 9:00
	expectOwnAppointment(e)
	sort := day(12, 9, 30)
	e.queue.EXPECT().GetQueueEntryForPatient(mock.Anything, db.GetQueueEntryForPatientParams{AppointmentID: puuid(apptID), PatientID: puuid(patient)}).Return(entryFor("waiting", sort), nil)
	e.queue.EXPECT().CountAhead(mock.Anything, db.CountAheadParams{DepartmentID: 1, ServiceDate: today(), SortTime: tsOf(sort)}).Return(int64(4), nil)
	e.admin.EXPECT().GetDepartment(mock.Anything, int64(1)).Return(std(10), nil)
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, mock.Anything).Return(int64(2), nil)
	e.queue.EXPECT().RecentConsultSeconds(mock.Anything, mock.Anything).Return(nil, nil)

	p, err := newWaitService(e).Position(context.Background(), patient, apptID)
	require.NoError(t, err)

	assert.Equal(t, "waiting", p.Status)
	assert.Equal(t, 4, p.AheadCount)
	require.NotNil(t, p.EstimatedCallAt)
	assert.True(t, p.EstimatedCallAt.Equal(clinicNow.Add(20*time.Minute)), "4 人 × 10 分 ÷ 2 医師 = 20 分後")
	assert.Equal(t, "standard", p.Basis)
}

// 見込みが算出できないときは null（数値を推測しない）
func TestPosition_NoEstimateWhenUnavailable(t *testing.T) {
	e := newClinicEnv(t)
	expectOwnAppointment(e)
	e.queue.EXPECT().GetQueueEntryForPatient(mock.Anything, mock.Anything).Return(entryFor("waiting", day(12, 9, 30)), nil)
	e.queue.EXPECT().CountAhead(mock.Anything, mock.Anything).Return(int64(2), nil)
	e.admin.EXPECT().GetDepartment(mock.Anything, int64(1)).Return(db.Department{ID: 1}, nil)
	e.queue.EXPECT().CountOnDutyDoctors(mock.Anything, mock.Anything).Return(int64(1), nil)
	e.queue.EXPECT().RecentConsultSeconds(mock.Anything, mock.Anything).Return(nil, nil)

	p, err := newWaitService(e).Position(context.Background(), patient, apptID)
	require.NoError(t, err)
	assert.Equal(t, 2, p.AheadCount, "人数は分かる")
	assert.Nil(t, p.EstimatedCallAt)
}

func TestPosition_NotCheckedIn(t *testing.T) {
	e := newClinicEnv(t)
	expectOwnAppointment(e)
	e.queue.EXPECT().GetQueueEntryForPatient(mock.Anything, mock.Anything).Return(db.QueueEntry{}, pgx.ErrNoRows)

	p, err := newWaitService(e).Position(context.Background(), patient, apptID)
	require.NoError(t, err)
	assert.Equal(t, "not_checked_in", p.Status)
	assert.Zero(t, p.AheadCount)
	assert.Nil(t, p.EstimatedCallAt)
}

func TestPosition_OtherStates(t *testing.T) {
	t.Run("呼出済み → いま呼ばれている", func(t *testing.T) {
		e := newClinicEnv(t)
		expectOwnAppointment(e)
		e.queue.EXPECT().GetQueueEntryForPatient(mock.Anything, mock.Anything).Return(entryFor("called", day(12, 8, 50)), nil)
		p, err := newWaitService(e).Position(context.Background(), patient, apptID)
		require.NoError(t, err)
		assert.Equal(t, "called", p.Status)
		assert.Zero(t, p.AheadCount)
		require.NotNil(t, p.EstimatedCallAt)
		assert.True(t, p.EstimatedCallAt.Equal(clinicNow))
	})
	for _, status := range []string{"in_consultation", "done", "canceled"} {
		t.Run(status, func(t *testing.T) {
			e := newClinicEnv(t)
			expectOwnAppointment(e)
			e.queue.EXPECT().GetQueueEntryForPatient(mock.Anything, mock.Anything).Return(entryFor(status, day(12, 8, 50)), nil)
			p, err := newWaitService(e).Position(context.Background(), patient, apptID)
			require.NoError(t, err)
			assert.Equal(t, status, p.Status)
			assert.Zero(t, p.AheadCount)
			assert.Nil(t, p.EstimatedCallAt)
		})
	}
}

// IDOR: 他人の予約の順番は見られない
func TestPosition_OtherPatientsAppointmentIsNotFound(t *testing.T) {
	e := newClinicEnv(t)
	e.appts.EXPECT().GetAppointment(mock.Anything, db.GetAppointmentParams{ID: apptID, PatientID: otherPat}).Return(db.Appointment{}, pgx.ErrNoRows)

	_, err := newWaitService(e).Position(context.Background(), otherPat, apptID)
	assert.ErrorIs(t, err, service.ErrAppointmentNotFound)
}
