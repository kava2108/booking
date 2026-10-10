package app_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
)

// フロントエンド対応: スタッフの当日予約一覧と、管理者画面の現在値の参照（実 DB・OpenAPI 契約チェック）

type staffApptJSON struct {
	ID            string `json:"id"`
	AppointmentNo string `json:"appointment_no"`
	SlotStart     string `json:"slot_start"`
	Doctor        string `json:"doctor"`
	PatientName   string `json:"patient_name"`
	VisitType     string `json:"visit_type"`
	Status        string `json:"status"`
	CheckedIn     bool   `json:"checked_in"`
}

func (a *clinicApp) staffAppointments(tok, query string) []staffApptJSON {
	a.t.Helper()
	rec := a.call(http.MethodGet, "/api/v1/staff/appointments?"+query, nil, tok)
	require.Equal(a.t, http.StatusOK, rec.Code, rec.Body.String())
	return jsonBody[[]staffApptJSON](a.t, rec)
}

// ---------- スタッフの当日予約一覧（来院受付の入口） ----------

func TestStaffAppointments_ListAndCheckInFlag(t *testing.T) {
	a := newClinicApp(t, nil)
	staff := a.staffTok()
	p1, p2 := a.newUser("p1@example.com", domain.RolePatient), a.newUser("p2@example.com", domain.RolePatient)
	t1, t2 := a.tokenFor(p1.ID, domain.RolePatient), a.tokenFor(p2.ID, domain.RolePatient)
	first := jsonBody[apptJSON](t, a.book(t1, a.seedSlot(day(12, 9, 30), 2)))
	a.book(t2, a.seedSlot(day(12, 10, 0), 2))
	canceled := jsonBody[apptJSON](t, a.book(t2, a.seedSlot(day(12, 11, 0), 2)))
	require.Equal(t, http.StatusNoContent, a.call(http.MethodDelete, "/api/v1/appointments/"+canceled.ID, nil, t2).Code)
	a.book(t1, a.seedSlot(day(13, 9, 30), 2)) // 翌日

	q := fmt.Sprintf("department_id=%d", a.deptID)
	got := a.staffAppointments(staff, q)

	require.Len(t, got, 2, "今日の有効な予約のみ（取消・翌日は除く）")
	assert.Equal(t, first.ID, got[0].ID)
	assert.Equal(t, "患者 p1@example.com", got[0].PatientName)
	assert.Equal(t, "山田", got[0].Doctor)
	assert.Equal(t, "2026-10-12T09:30:00+09:00", got[0].SlotStart)
	assert.Equal(t, "booked", got[0].Status)
	assert.False(t, got[0].CheckedIn)

	// 一覧の id でそのまま来院受付できる。受付後は checked_in=true、完了後は completed
	st := jsonBody[stateJSON](t, a.call(http.MethodPost, "/api/v1/staff/queue/"+got[0].ID+"/check-in", nil, staff))
	assert.True(t, a.staffAppointments(staff, q)[0].CheckedIn)
	for _, act := range []string{"call", "start", "complete"} {
		require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+st.ID+"/"+act, nil, staff).Code)
	}
	assert.Equal(t, "completed", a.staffAppointments(staff, q)[0].Status)

	// 日付指定で別の日も見られる
	assert.Len(t, a.staffAppointments(staff, q+"&date=2026-10-13"), 1)
	assert.Empty(t, a.staffAppointments(staff, q+"&date=2026-10-20"))
}

// 症状メモは来院受付の一覧に出さない（最小限の個人情報）
func TestStaffAppointments_DoesNotExposeSymptomNote(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	a.book(a.tokenFor(p.ID, domain.RolePatient), a.seedSlot(day(12, 9, 30), 1)) // 症状メモ「3日前から咳」つき

	rec := a.call(http.MethodGet, fmt.Sprintf("/api/v1/staff/appointments?department_id=%d", a.deptID), nil, a.staffTok())
	assert.NotContains(t, rec.Body.String(), "咳")
	assert.NotContains(t, rec.Body.String(), "symptom")
}

func TestStaffAppointments_AuthorizationAndAudit(t *testing.T) {
	a := newClinicApp(t, nil)
	path := fmt.Sprintf("/api/v1/staff/appointments?department_id=%d", a.deptID)

	assert.Equal(t, http.StatusUnauthorized, a.call(http.MethodGet, path, nil, "").Code)
	assert.Equal(t, http.StatusForbidden, a.call(http.MethodGet, path, nil, a.token(domain.RolePatient)).Code)
	assert.Equal(t, http.StatusForbidden, a.call(http.MethodGet, path, nil, a.token(domain.RoleSystemAdmin)).Code)
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM audit_logs`), "拒否されたアクセスは個人情報を返していないので記録しない")

	staff := a.staffTok()
	a.staffAppointments(staff, fmt.Sprintf("department_id=%d", a.deptID))
	var action, target string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT action, target FROM audit_logs`).Scan(&action, &target))
	assert.Equal(t, "view_staff_appointments", action)
	assert.Equal(t, fmt.Sprintf("department:%d", a.deptID), target)
}

func TestStaffAppointments_ValidationDoesNotWriteAudit(t *testing.T) {
	a := newClinicApp(t, nil)
	staff := a.staffTok()
	for name, q := range map[string]string{
		"department_id なし": "",
		"department_id 不正": "department_id=abc",
		"date の書式":         fmt.Sprintf("department_id=%d&date=10/12/2026", a.deptID),
		"存在しない日付":          fmt.Sprintf("department_id=%d&date=2026-02-30", a.deptID),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodGet, "/api/v1/staff/appointments?"+q, nil, staff).Code)
		})
	}
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM audit_logs`))
}

// ---------- 管理者画面: 現在の設定・スケジュールの参照 ----------

type settingsJSON struct {
	CancelDeadlineMinutes int `json:"cancel_deadline_minutes"`
	BookingWindowDays     int `json:"booking_window_days"`
	Departments           []struct {
		ID                     int64   `json:"id"`
		Name                   string  `json:"name"`
		StandardConsultMinutes *int    `json:"standard_consult_minutes"`
		WalkinCutoff           *string `json:"walkin_cutoff"`
		WalkinCapacity         *int    `json:"walkin_capacity"`
	} `json:"departments"`
}

func TestAdminGetSettings_ReflectsUpdates(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)

	rec := a.call(http.MethodGet, "/api/v1/admin/settings", nil, admin)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := jsonBody[settingsJSON](t, rec)
	assert.Equal(t, 60, got.CancelDeadlineMinutes, "既定値")
	assert.Equal(t, 30, got.BookingWindowDays)
	require.Len(t, got.Departments, 1)
	assert.Equal(t, "内科", got.Departments[0].Name)
	assert.Nil(t, got.Departments[0].WalkinCutoff, "未設定は null")
	assert.Nil(t, got.Departments[0].WalkinCapacity)

	require.Equal(t, http.StatusOK, a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{
		"cancel_deadline_minutes": 120, "booking_window_days": 14,
		"departments": []map[string]any{{"id": a.deptID, "standard_consult_minutes": 15, "walkin_cutoff": "11:30", "walkin_capacity": 20}},
	}, admin).Code)

	got = jsonBody[settingsJSON](t, a.call(http.MethodGet, "/api/v1/admin/settings", nil, admin))
	assert.Equal(t, 120, got.CancelDeadlineMinutes)
	assert.Equal(t, 14, got.BookingWindowDays)
	d := got.Departments[0]
	assert.Equal(t, 15, *d.StandardConsultMinutes)
	assert.Equal(t, "11:30", *d.WalkinCutoff)
	assert.Equal(t, 20, *d.WalkinCapacity)
}

func TestAdminGetSchedules_RoundTrip(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)
	a.putMondaySchedule(admin)
	path := fmt.Sprintf("/api/v1/admin/schedules?doctor_id=%d", a.doctorID)

	rec := a.call(http.MethodGet, path, nil, admin)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := jsonBody[struct {
		DoctorID  int64 `json:"doctor_id"`
		Schedules []struct {
			Weekday     int    `json:"weekday"`
			StartTime   string `json:"start_time"`
			EndTime     string `json:"end_time"`
			SlotMinutes int    `json:"slot_minutes"`
			Capacity    int    `json:"capacity"`
		} `json:"schedules"`
	}](t, rec)
	assert.Equal(t, a.doctorID, got.DoctorID)
	require.Len(t, got.Schedules, 1)
	assert.Equal(t, 1, got.Schedules[0].Weekday)
	assert.Equal(t, "09:00", got.Schedules[0].StartTime)
	assert.Equal(t, "12:00", got.Schedules[0].EndTime)
	assert.Equal(t, 30, got.Schedules[0].SlotMinutes)
	assert.Equal(t, 2, got.Schedules[0].Capacity)

	// スケジュールが無い医師は空配列
	var other int64
	require.NoError(t, a.pool.QueryRow(t.Context(), `INSERT INTO doctors(department_id,name) VALUES ($1,'佐藤') RETURNING id`, a.deptID).Scan(&other))
	rec = a.call(http.MethodGet, fmt.Sprintf("/api/v1/admin/schedules?doctor_id=%d", other), nil, admin)
	assert.Contains(t, rec.Body.String(), `"schedules":[]`)
}

func TestAdminGetEndpoints_AuthorizationAndValidation(t *testing.T) {
	a := newClinicApp(t, nil)
	for _, path := range []string{"/api/v1/admin/settings", fmt.Sprintf("/api/v1/admin/schedules?doctor_id=%d", a.doctorID)} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, a.call(http.MethodGet, path, nil, "").Code)
			assert.Equal(t, http.StatusForbidden, a.call(http.MethodGet, path, nil, a.token(domain.RolePatient)).Code)
			assert.Equal(t, http.StatusForbidden, a.call(http.MethodGet, path, nil, a.staffTok()).Code, "スタッフは設定を見られない")
			assert.Equal(t, http.StatusOK, a.call(http.MethodGet, path, nil, a.token(domain.RoleClinicAdmin)).Code)
		})
	}
	admin := a.token(domain.RoleClinicAdmin)
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodGet, "/api/v1/admin/schedules", nil, admin).Code, "doctor_id なし")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodGet, "/api/v1/admin/schedules?doctor_id=99999", nil, admin).Code, "未知の医師")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodGet, "/api/v1/admin/schedules?doctor_id=abc", nil, admin).Code)
}
