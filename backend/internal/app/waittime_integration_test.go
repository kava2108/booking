package app_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 15-clinic-reservation-wait-time-api TASK-0001: 待ち時間 API の統合テスト（実 DB・OpenAPI 契約チェック）

type deptWaitJSON struct {
	DepartmentID int64  `json:"department_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	WaitingCount *int   `json:"waiting_count"`
	WaitMinutes  *int   `json:"wait_minutes"`
	Basis        string `json:"basis"`
	Reason       string `json:"reason"`
}

type waitJSON struct {
	UpdatedAt   string         `json:"updated_at"`
	Note        string         `json:"note"`
	Departments []deptWaitJSON `json:"departments"`
}

// openClinic は月曜 8:00〜18:00 に診療する医師と、標準診察時間 10 分を設定する。
func (a *clinicApp) openClinic(standardMinutes int) {
	a.t.Helper()
	_, err := a.pool.Exec(a.t.Context(), `INSERT INTO doctor_schedules(doctor_id,weekday,start_time,end_time,slot_minutes,capacity) VALUES ($1,1,'08:00','18:00',30,2)`, a.doctorID)
	require.NoError(a.t, err)
	if standardMinutes > 0 {
		_, err = a.pool.Exec(a.t.Context(), `UPDATE departments SET standard_consult_minutes=$1`, standardMinutes)
	} else {
		_, err = a.pool.Exec(a.t.Context(), `UPDATE departments SET standard_consult_minutes=NULL`)
	}
	require.NoError(a.t, err)
}

func (a *clinicApp) waitTimes() (waitJSON, *httptest.ResponseRecorder) {
	a.t.Helper()
	rec := a.call(http.MethodGet, "/api/v1/wait-times", nil, "")
	require.Equal(a.t, http.StatusOK, rec.Code, rec.Body.String())
	return jsonBody[waitJSON](a.t, rec), rec
}

func (a *clinicApp) walkIns(n int) {
	a.t.Helper()
	staff := a.staffTok()
	for i := 0; i < n; i++ {
		require.Equal(a.t, http.StatusCreated, a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": fmt.Sprintf("秘密の患者%d", i)}, staff).Code)
		*a.clock = a.clock.Add(time.Second)
	}
}

// ---------- 公開 API (REQ-004-AC-1/2/6, REQ-007-AC-1) ----------

func TestWaitTimes_PublicWithStandardTime(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	a.walkIns(3)
	*a.clock = clinicNow.Add(time.Minute) // キャッシュ前の状態から進める

	w, rec := a.waitTimes()

	require.Len(t, w.Departments, 1)
	d := w.Departments[0]
	assert.Equal(t, "内科", d.Name)
	assert.Equal(t, "available", d.Status)
	require.NotNil(t, d.WaitingCount)
	assert.Equal(t, 3, *d.WaitingCount)
	require.NotNil(t, d.WaitMinutes)
	assert.Equal(t, 30, *d.WaitMinutes, "3 人 × 標準 10 分 ÷ 医師 1 人")
	assert.Equal(t, "standard", d.Basis)
	assert.Contains(t, w.Note, "目安", "目安である旨の注記を併記する (REQ-004-AC-6)")
	assert.NotEmpty(t, w.UpdatedAt)
	assert.Equal(t, "public, max-age=10", rec.Header().Get("Cache-Control"))
}

// 認証なしで見られ、個人を特定できる情報を含まない
func TestWaitTimes_NoPersonalData(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	a.walkIns(2)
	*a.clock = clinicNow.Add(time.Minute)

	_, rec := a.waitTimes()

	body := rec.Body.String()
	for _, leaked := range []string{"秘密の患者", "ticket_no", "patient", "symptom", "display_name", "email"} {
		assert.NotContains(t, body, leaked)
	}
	raw := jsonBody[map[string]any](t, rec)
	assert.ElementsMatch(t, []string{"updated_at", "note", "departments"}, mapKeys(raw))
	for _, d := range raw["departments"].([]any) {
		for k := range d.(map[string]any) {
			assert.Contains(t, []string{"department_id", "name", "status", "waiting_count", "wait_minutes", "basis", "reason"}, k)
		}
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// 実績が 3 件以上あれば actual（実績の平均で計算）
func TestWaitTimes_ActualBasisFromCompletedConsultations(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(30) // 標準は 30 分だが、実績(5 分)が優先される
	staff := a.staffTok()
	for i := 0; i < 3; i++ {
		rec := a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": fmt.Sprintf("x%d", i)}, staff)
		id := jsonBody[ticketJSON](t, rec).QueueEntryID
		for _, act := range []string{"call", "start", "complete"} {
			require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+id+"/"+act, nil, staff).Code)
		}
		_, err := a.pool.Exec(t.Context(), `UPDATE queue_entries SET started_at = completed_at - interval '5 minutes' WHERE id=$1`, id)
		require.NoError(t, err)
	}
	a.walkIns(2)
	*a.clock = clinicNow.Add(time.Minute)

	w, _ := a.waitTimes()
	d := w.Departments[0]
	assert.Equal(t, "actual", d.Basis)
	assert.Equal(t, 10, *d.WaitMinutes, "2 人 × 実績 5 分")
}

// REQ-004-AC-5: 算出できない場合は理由つきで「算出不可」。数値は返さない
func TestWaitTimes_UnavailableReasons(t *testing.T) {
	cases := map[string]struct {
		setup  func(a *clinicApp)
		reason string
	}{
		"診療開始前":   {func(a *clinicApp) { *a.clock = day(12, 7, 0) }, "before_hours"},
		"診療終了後":   {func(a *clinicApp) { *a.clock = day(12, 19, 0) }, "closed"},
		"休診日（日曜）": {func(a *clinicApp) { *a.clock = day(11, 10, 0) }, "closed"},
		"臨時休診中": {func(a *clinicApp) {
			_, err := a.pool.Exec(t.Context(), `INSERT INTO closures(doctor_id, period, reason) VALUES ($1, tstzrange('2026-10-12 09:00+09','2026-10-12 12:00+09'), '学会')`, a.doctorID)
			require.NoError(t, err)
		}, "closed"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := newClinicApp(t, nil)
			a.openClinic(10)
			c.setup(a)

			w, _ := a.waitTimes()
			d := w.Departments[0]
			assert.Equal(t, "unavailable", d.Status)
			assert.Equal(t, c.reason, d.Reason)
			assert.Nil(t, d.WaitMinutes, "数値を返さない")
			assert.Nil(t, d.WaitingCount)
			assert.Empty(t, d.Basis)
		})
	}
}

func TestWaitTimes_NoDataIsUnavailable(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(0) // 標準診察時間なし・実績なし
	a.walkIns(2)
	*a.clock = clinicNow.Add(time.Minute)

	w, _ := a.waitTimes()
	assert.Equal(t, "unavailable", w.Departments[0].Status)
	assert.Equal(t, "no_data", w.Departments[0].Reason)
	assert.Nil(t, w.Departments[0].WaitMinutes)
}

func TestWaitTimes_NobodyWaitingShowsZero(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(0)

	w, _ := a.waitTimes()
	d := w.Departments[0]
	assert.Equal(t, "available", d.Status)
	require.NotNil(t, d.WaitMinutes)
	assert.Equal(t, 0, *d.WaitMinutes)
	assert.Equal(t, 0, *d.WaitingCount)
}

// 10 秒キャッシュ: 期間内は変わらず、超えたら最新になる
func TestWaitTimes_CachedForTenSeconds(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	*a.clock = clinicNow.Add(time.Minute)
	first, _ := a.waitTimes()
	require.Equal(t, 0, *first.Departments[0].WaitingCount)

	require.Equal(t, http.StatusCreated, a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": "x"}, a.staffTok()).Code)

	*a.clock = a.clock.Add(9 * time.Second)
	cached, _ := a.waitTimes()
	assert.Equal(t, 0, *cached.Departments[0].WaitingCount, "TTL 内は更新されない")
	assert.Equal(t, first.UpdatedAt, cached.UpdatedAt)

	*a.clock = a.clock.Add(2 * time.Second)
	fresh, _ := a.waitTimes()
	assert.Equal(t, 1, *fresh.Departments[0].WaitingCount)
	assert.NotEqual(t, first.UpdatedAt, fresh.UpdatedAt)
}

func TestWaitTimes_PublicRateLimit(t *testing.T) {
	a := newClinicApp(t, func(c *config.Config) { c.RateLimitPublicMax = 2 })
	a.openClinic(10)

	assert.Equal(t, http.StatusOK, a.call(http.MethodGet, "/api/v1/wait-times", nil, "").Code)
	assert.Equal(t, http.StatusOK, a.call(http.MethodGet, "/api/v1/wait-times", nil, "").Code)
	rec := a.call(http.MethodGet, "/api/v1/wait-times", nil, "")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// ---------- 本人の順番 (REQ-004-AC-4) ----------

type positionJSON struct {
	Status          string  `json:"status"`
	AheadCount      int     `json:"ahead_count"`
	EstimatedCallAt *string `json:"estimated_call_at"`
	Basis           string  `json:"basis"`
}

func TestPosition_OwnerSeesAheadAndEstimate(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	p := a.newUser("p@example.com", domain.RolePatient)
	ptok := a.tokenFor(p.ID, domain.RolePatient)
	appt := jsonBody[apptJSON](t, a.book(ptok, a.seedSlot(day(12, 9, 30), 1)))
	path := "/api/v1/appointments/" + appt.ID + "/position"

	// 来院前
	rec := a.call(http.MethodGet, path, nil, ptok)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	pos := jsonBody[positionJSON](t, rec)
	assert.Equal(t, "not_checked_in", pos.Status)
	assert.Nil(t, pos.EstimatedCallAt)

	// 先に当日患者が 2 人並び、その後で来院受付（予約 9:30 > 現在 9:00 なので予約時刻で並ぶ → 当日患者より後ろ）
	a.walkIns(2)
	require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+appt.ID+"/check-in", nil, a.staffTok()).Code)
	*a.clock = a.clock.Add(time.Minute)

	rec = a.call(http.MethodGet, path, nil, ptok)
	require.Equal(t, http.StatusOK, rec.Code)
	pos = jsonBody[positionJSON](t, rec)
	assert.Equal(t, "waiting", pos.Status)
	assert.Equal(t, 2, pos.AheadCount)
	require.NotNil(t, pos.EstimatedCallAt)
	assert.Equal(t, "standard", pos.Basis)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestPosition_ProgressesWithQueueState(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	p := a.newUser("p@example.com", domain.RolePatient)
	ptok := a.tokenFor(p.ID, domain.RolePatient)
	appt := jsonBody[apptJSON](t, a.book(ptok, a.seedSlot(day(12, 9, 30), 1)))
	staff := a.staffTok()
	q := jsonBody[stateJSON](t, a.call(http.MethodPost, "/api/v1/staff/queue/"+appt.ID+"/check-in", nil, staff))
	path := "/api/v1/appointments/" + appt.ID + "/position"

	for _, step := range []struct{ action, status string }{{"", "waiting"}, {"call", "called"}, {"start", "in_consultation"}, {"complete", "done"}} {
		if step.action != "" {
			require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+q.ID+"/"+step.action, nil, staff).Code)
		}
		pos := jsonBody[positionJSON](t, a.call(http.MethodGet, path, nil, ptok))
		assert.Equal(t, step.status, pos.Status)
		assert.Equal(t, 0, pos.AheadCount)
	}
}

// IDOR: 他人の予約の順番は見られない（存在しない ID と同じ 404）
func TestPosition_AuthAndOwnership(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	owner, other := a.newUser("o@example.com", domain.RolePatient), a.newUser("x@example.com", domain.RolePatient)
	appt := jsonBody[apptJSON](t, a.book(a.tokenFor(owner.ID, domain.RolePatient), a.seedSlot(day(12, 9, 30), 1)))
	path := "/api/v1/appointments/" + appt.ID + "/position"

	assert.Equal(t, http.StatusUnauthorized, a.call(http.MethodGet, path, nil, "").Code)
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodGet, path, nil, a.tokenFor(other.ID, domain.RolePatient)).Code)
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodGet, "/api/v1/appointments/"+uuid.NewString()+"/position", nil, a.tokenFor(owner.ID, domain.RolePatient)).Code)
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodGet, "/api/v1/appointments/not-a-uuid/position", nil, a.tokenFor(owner.ID, domain.RolePatient)).Code)
}
