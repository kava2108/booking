package app_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 13-clinic-reservation-queue-staff-walkin TASK-0001: 待機列・スタッフ操作・当日受付の統合テスト

type stateJSON struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ticketJSON struct {
	QueueEntryID string `json:"queue_entry_id"`
	TicketNo     int    `json:"ticket_no"`
	AheadCount   int    `json:"ahead_count"`
}

type staffEntryJSON struct {
	ID          string `json:"id"`
	TicketNo    int    `json:"ticket_no"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	PatientName string `json:"patient_name"`
	SymptomNote string `json:"symptom_note"`
}

// staffTok は実在のスタッフユーザーのトークン（監査ログの actor_id は users への外部キーのため）。
func (a *clinicApp) staffTok() string {
	if a.staffUser == nil {
		u := a.newUser("staff@example.com", domain.RoleStaff)
		a.staffUser = &u
	}
	return a.tokenFor(a.staffUser.ID, domain.RoleStaff)
}

func (a *clinicApp) queue(tok string) []staffEntryJSON {
	a.t.Helper()
	rec := a.call(http.MethodGet, fmt.Sprintf("/api/v1/staff/queue?department_id=%d", a.deptID), nil, tok)
	require.Equal(a.t, http.StatusOK, rec.Code, rec.Body.String())
	return jsonBody[[]staffEntryJSON](a.t, rec)
}

// ---------- 来院受付 → 呼出 → 診察開始 → 完了 (REQ-005-AC-1/2) ----------

func TestQueue_ReservationLifecycle(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	slot := a.seedSlot(day(12, 9, 30), 1) // 本日 9:30（現在 9:00）
	appt := jsonBody[apptJSON](t, a.book(a.tokenFor(p.ID, domain.RolePatient), slot))
	staff := a.staffTok()

	rec := a.call(http.MethodPost, "/api/v1/staff/queue/"+appt.ID+"/check-in", nil, staff)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	st := jsonBody[stateJSON](t, rec)
	assert.Equal(t, "waiting", st.Status)

	// スタッフの一覧には氏名と症状メモが見える
	list := a.queue(staff)
	require.Len(t, list, 1)
	assert.Equal(t, "reservation", list[0].Source)
	assert.Equal(t, "患者 p@example.com", list[0].PatientName)
	assert.Equal(t, "3日前から咳", list[0].SymptomNote)

	for _, step := range []struct{ action, status string }{{"call", "called"}, {"start", "in_consultation"}, {"complete", "done"}} {
		rec = a.call(http.MethodPost, "/api/v1/staff/queue/"+st.ID+"/"+step.action, nil, staff)
		require.Equal(t, http.StatusOK, rec.Code, step.action+": "+rec.Body.String())
		assert.Equal(t, step.status, jsonBody[stateJSON](t, rec).Status)
	}

	assert.Empty(t, a.queue(staff), "完了すると列から消える")
	assert.Equal(t, "completed", func() string {
		var s string
		require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT status FROM appointments WHERE id=$1`, appt.ID).Scan(&s))
		return s
	}(), "予約も completed になる")
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM queue_entries WHERE status='done' AND started_at IS NOT NULL AND completed_at IS NOT NULL`), "所要時間の元データが記録される")
}

func TestQueue_InvalidTransitionsAndMissing(t *testing.T) {
	a := newClinicApp(t, nil)
	staff := a.staffTok()
	rec := a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": "電話 太郎"}, staff)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := jsonBody[ticketJSON](t, rec).QueueEntryID

	for _, action := range []string{"start", "complete"} {
		rec := a.call(http.MethodPost, "/api/v1/staff/queue/"+id+"/"+action, nil, staff)
		assert.Equal(t, http.StatusConflict, rec.Code, "waiting から "+action)
		body := jsonBody[map[string]any](t, rec)
		assert.Equal(t, "invalid_transition", body["error"])
		assert.Contains(t, body["message"], "waiting", "現在の状態を伝える")
	}
	require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+id+"/call", nil, staff).Code)
	assert.Equal(t, http.StatusConflict, a.call(http.MethodPost, "/api/v1/staff/queue/"+id+"/call", nil, staff).Code, "二重呼出")

	assert.Equal(t, http.StatusNotFound, a.call(http.MethodPost, "/api/v1/staff/queue/"+uuid.NewString()+"/call", nil, staff).Code)
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodPost, "/api/v1/staff/queue/not-a-uuid/call", nil, staff).Code)
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodPost, "/api/v1/staff/queue/"+uuid.NewString()+"/check-in", nil, staff).Code, "存在しない予約")
}

func TestQueue_CheckInRules(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	ptok := a.tokenFor(p.ID, domain.RolePatient)
	staff := a.staffTok()

	today := jsonBody[apptJSON](t, a.book(ptok, a.seedSlot(day(12, 9, 30), 1)))
	tomorrow := jsonBody[apptJSON](t, a.book(ptok, a.seedSlot(day(13, 9, 30), 1)))
	canceled := jsonBody[apptJSON](t, a.book(ptok, a.seedSlot(day(19, 9, 30), 1)))
	require.Equal(t, http.StatusNoContent, a.call(http.MethodDelete, "/api/v1/appointments/"+canceled.ID, nil, ptok).Code)

	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPost, "/api/v1/staff/queue/"+tomorrow.ID+"/check-in", nil, staff).Code, "当日でない")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPost, "/api/v1/staff/queue/"+canceled.ID+"/check-in", nil, staff).Code, "取消済み")
	assert.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+today.ID+"/check-in", nil, staff).Code)
	assert.Equal(t, http.StatusConflict, a.call(http.MethodPost, "/api/v1/staff/queue/"+today.ID+"/check-in", nil, staff).Code, "二重の来院受付")
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM queue_entries`))
}

// 遅刻した予約患者は、先に当日受付した人を追い越さない
func TestQueue_LateReservationDoesNotOvertakeWalkIn(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("late@example.com", domain.RolePatient)
	staff := a.staffTok()
	*a.clock = day(12, 8, 0)
	slot := a.seedSlot(day(12, 8, 30), 1) // 8:30 の予約
	appt := jsonBody[apptJSON](t, a.book(a.tokenFor(p.ID, domain.RolePatient), slot))

	*a.clock = day(12, 8, 40)
	require.Equal(t, http.StatusCreated, a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": "先着の当日患者"}, staff).Code)
	*a.clock = day(12, 8, 50) // 予約患者は 20 分遅刻して到着
	require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+appt.ID+"/check-in", nil, staff).Code)

	list := a.queue(staff)
	require.Len(t, list, 2)
	assert.Equal(t, "先着の当日患者", list[0].PatientName)
	assert.Equal(t, "reservation", list[1].Source)
}

// ---------- 当日受付 (REQ-008) ----------

func TestWalkIn_PatientSelfRegistration(t *testing.T) {
	a := newClinicApp(t, nil)
	p1, p2 := a.newUser("p1@example.com", domain.RolePatient), a.newUser("p2@example.com", domain.RolePatient)

	rec := a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{"department_id": a.deptID}, a.tokenFor(p1.ID, domain.RolePatient))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	t1 := jsonBody[ticketJSON](t, rec)
	assert.Equal(t, 1, t1.TicketNo)
	assert.Equal(t, 0, t1.AheadCount)

	*a.clock = a.clock.Add(time.Minute)
	t2 := jsonBody[ticketJSON](t, a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{"department_id": a.deptID}, a.tokenFor(p2.ID, domain.RolePatient)))
	assert.Equal(t, 2, t2.TicketNo)
	assert.Equal(t, 1, t2.AheadCount, "受付順で 1 人待ち")

	// 同じ患者は同じ診療科に二重に並べない
	rec = a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{"department_id": a.deptID}, a.tokenFor(p1.ID, domain.RolePatient))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "already_in_queue", errCode(t, rec))
}

func TestWalkIn_Validation(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)

	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{}, tok).Code)
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{"department_id": 99999}, tok).Code, "存在しない診療科")
	assert.Equal(t, http.StatusUnauthorized, a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{"department_id": a.deptID}, "").Code)

	staff := a.staffTok()
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID}, staff).Code, "氏名も患者も無い")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "patient_id": "x"}, staff).Code)
}

// REQ-008-AC-3: 締切後・定員到達は拒否し、理由を伝える
func TestWalkIn_CutoffAndCapacity(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)
	require.Equal(t, http.StatusOK, a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{
		"departments": []map[string]any{{"id": a.deptID, "walkin_cutoff": "09:30", "walkin_capacity": 2}},
	}, admin).Code)
	staff := a.staffTok()
	walkIn := func(name string) *httptest.ResponseRecorder {
		return a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": name}, staff)
	}

	assert.Equal(t, http.StatusCreated, walkIn("1人目").Code)
	assert.Equal(t, http.StatusCreated, walkIn("2人目").Code)
	rec := walkIn("3人目")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "定員 2 に到達")
	assert.Equal(t, "walkin_closed", errCode(t, rec))
	assert.Contains(t, jsonBody[map[string]any](t, rec)["message"], "capacity")

	// 締切時刻 9:30 ちょうどは拒否（定員が空いていても）
	*a.clock = day(12, 9, 30)
	_, err := a.pool.Exec(t.Context(), `UPDATE queue_entries SET status='canceled'`)
	require.NoError(t, err)
	rec = walkIn("締切後")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, jsonBody[map[string]any](t, rec)["message"], "cutoff")
}

// 同時の当日受付でも受付番号が重複せず、定員を超えない
func TestWalkIn_ConcurrentRegistrationsRespectCapacityAndNumbering(t *testing.T) {
	a := newClinicApp(t, nil)
	require.Equal(t, http.StatusOK, a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{
		"departments": []map[string]any{{"id": a.deptID, "walkin_capacity": 5}},
	}, a.token(domain.RoleClinicAdmin)).Code)
	staff := a.staffTok()

	var ok, closed atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := a.callQuiet(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": fmt.Sprintf("p%d", i)}, staff, start)
			switch rec.Code {
			case http.StatusCreated:
				ok.Add(1)
			case http.StatusUnprocessableEntity:
				closed.Add(1)
			default:
				t.Errorf("unexpected %d: %s", rec.Code, rec.Body.String())
			}
		}(i)
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, 5, ok.Load(), "定員 5 を超えない")
	assert.EqualValues(t, 10, closed.Load())
	assert.Equal(t, 5, a.count(`SELECT count(DISTINCT ticket_no) FROM queue_entries`), "受付番号が重複しない")
	assert.Equal(t, 5, a.count(`SELECT max(ticket_no) FROM queue_entries`), "1〜5 の連番")
}

// ---------- 認可・監査 (REQ-005-AC-4, REQ-007-AC-2/3) ----------

func TestStaffEndpoints_Authorization(t *testing.T) {
	a := newClinicApp(t, nil)
	id := uuid.NewString()
	endpoints := []struct{ method, path string }{
		{http.MethodGet, fmt.Sprintf("/api/v1/staff/queue?department_id=%d", a.deptID)},
		{http.MethodPost, "/api/v1/staff/walk-ins"},
		{http.MethodPost, "/api/v1/staff/queue/" + id + "/check-in"},
		{http.MethodPost, "/api/v1/staff/queue/" + id + "/call"},
		{http.MethodPost, "/api/v1/staff/queue/" + id + "/start"},
		{http.MethodPost, "/api/v1/staff/queue/" + id + "/complete"},
	}
	body := map[string]any{"department_id": a.deptID, "display_name": "x"}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, a.call(ep.method, ep.path, body, "").Code, "トークンなし")
			assert.Equal(t, http.StatusForbidden, a.call(ep.method, ep.path, body, a.token(domain.RolePatient)).Code, "患者")
			assert.Equal(t, http.StatusForbidden, a.call(ep.method, ep.path, body, a.token(domain.RoleSystemAdmin)).Code, "システム管理者（個人情報を見られない）")
			assert.NotEqual(t, http.StatusForbidden, a.call(ep.method, ep.path, body, a.staffTok()).Code, "スタッフ")
			assert.NotEqual(t, http.StatusForbidden, a.call(ep.method, ep.path, body, a.tokenFor(a.newUser(uuid.NewString()+"@example.com", domain.RoleClinicAdmin).ID, domain.RoleClinicAdmin)).Code, "クリニック管理者")
		})
	}
}

func TestStaffQueue_ViewIsAuditedAndRequiresDepartment(t *testing.T) {
	a := newClinicApp(t, nil)
	staffUser := a.newUser("staff@example.com", domain.RoleStaff)
	tok := a.tokenFor(staffUser.ID, domain.RoleStaff)

	a.queue(tok)

	var action, target string
	var actor uuid.UUID
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT action, target, actor_id FROM audit_logs`).Scan(&action, &target, &actor))
	assert.Equal(t, "view_staff_queue", action)
	assert.Equal(t, fmt.Sprintf("department:%d", a.deptID), target)
	assert.Equal(t, staffUser.ID, actor)

	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodGet, "/api/v1/staff/queue", nil, tok).Code)
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM audit_logs`), "検証エラーでは記録しない（個人情報を返していない）")
}

// 患者向けの応答・ログに他人の氏名や症状メモが出ない
func TestQueue_NoPersonalDataLeaksToPatientsOrLogs(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("secret@example.com", domain.RolePatient)
	appt := jsonBody[apptJSON](t, a.book(a.tokenFor(p.ID, domain.RolePatient), a.seedSlot(day(12, 9, 30), 1)))
	require.Equal(t, http.StatusOK, a.call(http.MethodPost, "/api/v1/staff/queue/"+appt.ID+"/check-in", nil, a.staffTok()).Code)

	other := a.newUser("other@example.com", domain.RolePatient)
	rec := a.call(http.MethodPost, "/api/v1/walk-ins", map[string]any{"department_id": a.deptID}, a.tokenFor(other.ID, domain.RolePatient))
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret@example.com")
	assert.NotContains(t, rec.Body.String(), "咳")
	assert.NotContains(t, a.logs.String(), "3日前から咳", "ログに症状メモを出さない")
}

// S4: スタッフの状態変更・代行受付と、管理者の設定変更も監査ログに残る(誰が・どの対象に)
func TestStaffAndAdminWrites_AreAudited(t *testing.T) {
	a := newClinicApp(t, nil)
	staffUser := a.newUser("staff@example.com", domain.RoleStaff)
	adminUser := a.newUser("admin@example.com", domain.RoleClinicAdmin)

	rec := a.call(http.MethodPost, "/api/v1/staff/walk-ins", map[string]any{"department_id": a.deptID, "display_name": "来院 太郎"}, a.tokenFor(staffUser.ID, domain.RoleStaff))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{"cancel_deadline_minutes": 90}, a.tokenFor(adminUser.ID, domain.RoleClinicAdmin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var action, target string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT action, target FROM audit_logs WHERE actor_id=$1`, staffUser.ID).Scan(&action, &target))
	assert.Equal(t, "staff_queue_action", action)
	assert.Equal(t, "/api/v1/staff/walk-ins", target)
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT action, target FROM audit_logs WHERE actor_id=$1`, adminUser.ID).Scan(&action, &target))
	assert.Equal(t, "admin_update", action)
	assert.Equal(t, "/api/v1/admin/settings", target)

	// 認可で弾かれた要求は記録しない
	before := a.count(`SELECT count(*) FROM audit_logs`)
	assert.Equal(t, http.StatusForbidden, a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{"cancel_deadline_minutes": 10}, a.token(domain.RolePatient)).Code)
	assert.Equal(t, before, a.count(`SELECT count(*) FROM audit_logs`))
}
