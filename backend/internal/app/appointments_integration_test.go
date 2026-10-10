package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 12-clinic-reservation-appointment-api TASK-0001: 予約 API の統合テスト（実 DB・OpenAPI 契約チェック）

// seedSlot は指定時刻の枠を作る（ID を返す）。
func (a *clinicApp) seedSlot(start time.Time, capacity int) uuid.UUID {
	a.t.Helper()
	var id uuid.UUID
	require.NoError(a.t, a.pool.QueryRow(a.t.Context(),
		`INSERT INTO slots(department_id, doctor_id, start_at, capacity) VALUES ($1,$2,$3,$4) RETURNING id`,
		a.deptID, a.doctorID, start, capacity).Scan(&id))
	return id
}

func (a *clinicApp) bookedCount(slot uuid.UUID) int {
	var n int
	require.NoError(a.t, a.pool.QueryRow(a.t.Context(), `SELECT booked_count FROM slots WHERE id=$1`, slot).Scan(&n))
	return n
}

func (a *clinicApp) count(query string, args ...any) int {
	var n int
	require.NoError(a.t, a.pool.QueryRow(a.t.Context(), query, args...).Scan(&n))
	return n
}

type apptJSON struct {
	ID            string `json:"id"`
	AppointmentNo string `json:"appointment_no"`
	SlotStart     string `json:"slot_start"`
	Department    string `json:"department"`
	Doctor        string `json:"doctor"`
	VisitType     string `json:"visit_type"`
	Status        string `json:"status"`
	SymptomNote   string `json:"symptom_note"`
	Cancelable    bool   `json:"cancelable"`
}

func (a *clinicApp) book(token string, slot uuid.UUID) *httptest.ResponseRecorder {
	return a.call(http.MethodPost, "/api/v1/appointments", map[string]any{"slot_id": slot.String(), "visit_type": "revisit", "symptom_note": "3日前から咳"}, token)
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	return jsonBody[map[string]any](t, rec)["error"].(string)
}

// ---------- 予約確定・一覧 (REQ-002-AC-1/3/4, REQ-003-AC-1) ----------

func TestAppointments_CreateAndList(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p1@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	slot := a.seedSlot(day(19, 9, 30), 2)

	start := time.Now()
	rec := a.book(tok, slot)
	elapsed := time.Since(start)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Less(t, elapsed, 3*time.Second, "予約確定は 3 秒以内 (REQ-002-AC-4)")

	got := jsonBody[apptJSON](t, rec)
	assert.Regexp(t, `^A-20261019-\d{4}$`, got.AppointmentNo)
	assert.Equal(t, "2026-10-19T09:30:00+09:00", got.SlotStart)
	assert.Equal(t, "内科", got.Department)
	assert.Equal(t, "山田", got.Doctor)
	assert.Equal(t, 1, a.bookedCount(slot))

	// 確認通知は同じ Tx でアウトボックスに入る。症状メモは含まれない
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM notifications WHERE kind='appointment_confirmed' AND status='pending' AND user_id=$1`, p.ID))
	var payload string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT payload::text FROM notifications`).Scan(&payload))
	assert.Contains(t, payload, got.AppointmentNo)
	assert.NotContains(t, payload, "咳")
	assert.NotContains(t, payload, "symptom")

	// 一覧: 自分の予約のみ。症状メモは本人にだけ返る
	rec = a.call(http.MethodGet, "/api/v1/appointments/me", nil, tok)
	require.Equal(t, http.StatusOK, rec.Code)
	list := jsonBody[[]apptJSON](t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, "booked", list[0].Status)
	assert.Equal(t, "3日前から咳", list[0].SymptomNote)
	assert.True(t, list[0].Cancelable)

	other := a.newUser("p2@example.com", domain.RolePatient)
	rec = a.call(http.MethodGet, "/api/v1/appointments/me", nil, a.tokenFor(other.ID, domain.RolePatient))
	assert.Empty(t, jsonBody[[]apptJSON](t, rec), "他人の予約は見えない")
	assert.NotContains(t, rec.Body.String(), "咳")
}

func TestAppointments_RequireAuth(t *testing.T) {
	a := newClinicApp(t, nil)
	id := uuid.NewString()
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/appointments"},
		{http.MethodGet, "/api/v1/appointments/me"},
		{http.MethodPatch, "/api/v1/appointments/" + id},
		{http.MethodDelete, "/api/v1/appointments/" + id},
	} {
		assert.Equal(t, http.StatusUnauthorized, a.call(c.method, c.path, map[string]any{}, "").Code, c.method+" "+c.path)
		assert.Equal(t, http.StatusUnauthorized, a.call(c.method, c.path, map[string]any{}, "garbage").Code)
	}
}

// ---------- 競合 (REQ-002-AC-2/5) ----------

func TestAppointments_SlotFullReturns409AndLeavesNoTrace(t *testing.T) {
	a := newClinicApp(t, nil)
	p1, p2 := a.newUser("p1@example.com", domain.RolePatient), a.newUser("p2@example.com", domain.RolePatient)
	slot := a.seedSlot(day(19, 9, 30), 1)

	require.Equal(t, http.StatusCreated, a.book(a.tokenFor(p1.ID, domain.RolePatient), slot).Code)
	rec := a.book(a.tokenFor(p2.ID, domain.RolePatient), slot)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "slot_full", errCode(t, rec))
	assert.Equal(t, 1, a.bookedCount(slot))
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM appointments`))
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM notifications`), "失敗した予約の通知は残らない")
}

// 20 人が同時に capacity=1 の枠を予約しても、成功は 1 件のみ
func TestAppointments_ConcurrentBookingNeverOverbooks(t *testing.T) {
	a := newClinicApp(t, nil)
	slot := a.seedSlot(day(19, 9, 30), 1)
	const n = 20
	tokens := make([]string, n)
	for i := range tokens {
		u := a.newUser(fmt.Sprintf("c%d@example.com", i), domain.RolePatient)
		tokens[i] = a.tokenFor(u.ID, domain.RolePatient)
	}

	var created, full, other atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, tok := range tokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"slot_id": slot.String(), "visit_type": "first"})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok)
			req.RemoteAddr = "203.0.113.30:1"
			rec := httptest.NewRecorder()
			<-start
			a.e.ServeHTTP(rec, req)
			switch rec.Code {
			case http.StatusCreated:
				created.Add(1)
			case http.StatusConflict:
				if strings.Contains(rec.Body.String(), "slot_full") {
					full.Add(1)
				} else {
					other.Add(1)
				}
			default:
				other.Add(1)
				t.Errorf("unexpected %d: %s", rec.Code, rec.Body.String())
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, 1, created.Load())
	assert.EqualValues(t, n-1, full.Load())
	assert.EqualValues(t, 0, other.Load())
	assert.Equal(t, 1, a.bookedCount(slot))
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM appointments WHERE status='booked'`))
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM notifications`))
}

// 同じ患者が同じ時刻に二重に予約できない（同一枠・同時刻の別枠の両方）
func TestAppointments_DuplicateForSamePatient(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	slot := a.seedSlot(day(19, 9, 30), 2)
	var otherDoc int64
	require.NoError(t, a.pool.QueryRow(t.Context(), `INSERT INTO doctors(department_id,name) VALUES ($1,'佐藤') RETURNING id`, a.deptID).Scan(&otherDoc))
	var sameTimeOtherDoctor uuid.UUID
	require.NoError(t, a.pool.QueryRow(t.Context(), `INSERT INTO slots(department_id,doctor_id,start_at,capacity) VALUES ($1,$2,$3,2) RETURNING id`, a.deptID, otherDoc, day(19, 9, 30)).Scan(&sameTimeOtherDoctor))

	require.Equal(t, http.StatusCreated, a.book(tok, slot).Code)

	for name, s := range map[string]uuid.UUID{"同じ枠": slot, "同時刻の別医師の枠": sameTimeOtherDoctor} {
		rec := a.book(tok, s)
		assert.Equal(t, http.StatusConflict, rec.Code, name)
		assert.Equal(t, "duplicate_appointment", errCode(t, rec), name)
	}
	assert.Equal(t, 1, a.bookedCount(slot), "重複で枠のカウントが増えない")
}

// 予約作成に失敗（ユーザーが存在しない → FK 違反）したら、確保した枠も巻き戻る
func TestAppointments_FailureAfterReserveRollsBackSlot(t *testing.T) {
	a := newClinicApp(t, nil)
	slot := a.seedSlot(day(19, 9, 30), 2)
	ghost := a.tokenFor(uuid.New(), domain.RolePatient) // JWT は有効だが users に存在しない

	rec := a.book(ghost, slot)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "internal_error", errCode(t, rec))
	assert.NotContains(t, rec.Body.String(), "foreign key", "内部エラーの詳細を返さない")
	assert.Equal(t, 0, a.bookedCount(slot), "枠の確保が巻き戻る")
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM notifications`))
}

// ---------- 入力検証 ----------

func TestAppointments_Validation(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	future := a.seedSlot(day(19, 9, 30), 1)
	past := a.seedSlot(day(12, 8, 0), 1)
	now := a.seedSlot(day(12, 9, 0), 1)
	farFuture := a.seedSlot(day(12, 9, 0).AddDate(0, 0, 60), 1)
	closed := a.seedSlot(day(19, 10, 0), 1)
	_, err := a.pool.Exec(t.Context(), `UPDATE slots SET status='closed' WHERE id=$1`, closed)
	require.NoError(t, err)

	cases := map[string]map[string]any{
		"visit_type 不正":      {"slot_id": future.String(), "visit_type": "other"},
		"visit_type なし":      {"slot_id": future.String()},
		"slot_id なし":         {"visit_type": "first"},
		"slot_id が UUID でない": {"slot_id": "abc", "visit_type": "first"},
		"症状メモが長すぎる":          {"slot_id": future.String(), "visit_type": "first", "symptom_note": strings.Repeat("あ", 1001)},
		"存在しない枠":             {"slot_id": uuid.NewString(), "visit_type": "first"},
		"過去の枠":               {"slot_id": past.String(), "visit_type": "first"},
		"現在ちょうどの枠":           {"slot_id": now.String(), "visit_type": "first"},
		"受付期間外の枠":            {"slot_id": farFuture.String(), "visit_type": "first"},
		"休診の枠":               {"slot_id": closed.String(), "visit_type": "first"},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := a.call(http.MethodPost, "/api/v1/appointments", body, tok)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			assert.Equal(t, "validation_failed", errCode(t, rec))
		})
	}
	assert.Equal(t, http.StatusBadRequest, a.call(http.MethodPost, "/api/v1/appointments", "{oops", tok).Code)
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM appointments`))

	// 境界: 症状メモちょうど 1000 文字は可
	rec := a.call(http.MethodPost, "/api/v1/appointments", map[string]any{"slot_id": future.String(), "visit_type": "first", "symptom_note": strings.Repeat("あ", 1000)}, tok)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

// ---------- キャンセル (REQ-003-AC-2/3) ----------

func TestAppointments_Cancel(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	slot := a.seedSlot(day(19, 9, 30), 1)
	appt := jsonBody[apptJSON](t, a.book(tok, slot))

	rec := a.call(http.MethodDelete, "/api/v1/appointments/"+appt.ID, nil, tok)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	assert.Equal(t, 0, a.bookedCount(slot), "枠が解放される")
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM notifications WHERE kind='appointment_canceled'`))
	list := jsonBody[[]apptJSON](t, a.call(http.MethodGet, "/api/v1/appointments/me", nil, tok))
	assert.Equal(t, "canceled", list[0].Status)
	assert.False(t, list[0].Cancelable)

	assert.Equal(t, http.StatusNotFound, a.call(http.MethodDelete, "/api/v1/appointments/"+appt.ID, nil, tok).Code, "二重キャンセルは 404")

	// 解放された枠に別の患者が予約できる
	p2 := a.newUser("p2@example.com", domain.RolePatient)
	assert.Equal(t, http.StatusCreated, a.book(a.tokenFor(p2.ID, domain.RolePatient), slot).Code)
}

func TestAppointments_CancelAfterDeadline(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	slot := a.seedSlot(day(12, 9, 30), 1) // 現在 9:00、開始は 30 分後。期限は 60 分前 → 既に過ぎている

	// 予約自体は可能（開始前）。キャンセルは期限超過で不可
	appt := jsonBody[apptJSON](t, a.book(tok, slot))
	rec := a.call(http.MethodDelete, "/api/v1/appointments/"+appt.ID, nil, tok)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "cancel_deadline_passed", errCode(t, rec))
	assert.Equal(t, 1, a.bookedCount(slot), "期限超過では枠を解放しない")

	list := jsonBody[[]apptJSON](t, a.call(http.MethodGet, "/api/v1/appointments/me", nil, tok))
	assert.False(t, list[0].Cancelable)
}

// 管理者が設定したキャンセル期限が反映される
func TestAppointments_CancelDeadlineFollowsSettings(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	slot := a.seedSlot(day(12, 9, 30), 1)
	appt := jsonBody[apptJSON](t, a.book(tok, slot))

	_, err := a.pool.Exec(t.Context(), `UPDATE clinic_settings SET cancel_deadline_minutes = 10`)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, a.call(http.MethodDelete, "/api/v1/appointments/"+appt.ID, nil, tok).Code, "期限 10 分なら 30 分前はキャンセルできる")
}

// IDOR: 他人の予約は取消・変更できず、存在も分からない（404）
func TestAppointments_OtherPatientsAppointmentIs404(t *testing.T) {
	a := newClinicApp(t, nil)
	owner, intruder := a.newUser("o@example.com", domain.RolePatient), a.newUser("i@example.com", domain.RolePatient)
	slot, slot2 := a.seedSlot(day(19, 9, 30), 1), a.seedSlot(day(19, 10, 0), 1)
	appt := jsonBody[apptJSON](t, a.book(a.tokenFor(owner.ID, domain.RolePatient), slot))
	itok := a.tokenFor(intruder.ID, domain.RolePatient)

	rec := a.call(http.MethodDelete, "/api/v1/appointments/"+appt.ID, nil, itok)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = a.call(http.MethodPatch, "/api/v1/appointments/"+appt.ID, map[string]any{"slot_id": slot2.String()}, itok)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodDelete, "/api/v1/appointments/"+uuid.NewString(), nil, itok).Code, "存在しない ID と区別できない")
	assert.Equal(t, http.StatusNotFound, a.call(http.MethodDelete, "/api/v1/appointments/not-a-uuid", nil, itok).Code)

	assert.Equal(t, 1, a.bookedCount(slot))
	assert.Equal(t, "booked", func() string {
		var s string
		require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT status FROM appointments WHERE id=$1`, appt.ID).Scan(&s))
		return s
	}())
}

// ---------- 日時変更 (REQ-003-AC-4) ----------

func TestAppointments_Reschedule(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	oldSlot, newSlot := a.seedSlot(day(19, 9, 30), 1), a.seedSlot(day(26, 10, 0), 1)
	appt := jsonBody[apptJSON](t, a.book(tok, oldSlot))

	rec := a.call(http.MethodPatch, "/api/v1/appointments/"+appt.ID, map[string]any{"slot_id": newSlot.String()}, tok)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := jsonBody[apptJSON](t, rec)
	assert.Equal(t, appt.ID, got.ID)
	assert.Equal(t, appt.AppointmentNo, got.AppointmentNo, "予約番号は変わらない")
	assert.Equal(t, "2026-10-26T10:00:00+09:00", got.SlotStart)
	assert.Equal(t, 0, a.bookedCount(oldSlot), "旧枠が解放される")
	assert.Equal(t, 1, a.bookedCount(newSlot), "新枠が確保される")

	var payload string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT payload::text FROM notifications WHERE kind='appointment_rescheduled'`).Scan(&payload))
	assert.Contains(t, payload, "previous_start_at")
	assert.NotContains(t, payload, "咳")
}

// 新しい枠が満席なら変更に失敗し、元の予約はそのまま残る
func TestAppointments_RescheduleToFullSlotKeepsOriginal(t *testing.T) {
	a := newClinicApp(t, nil)
	p1, p2 := a.newUser("p1@example.com", domain.RolePatient), a.newUser("p2@example.com", domain.RolePatient)
	tok1 := a.tokenFor(p1.ID, domain.RolePatient)
	oldSlot, fullSlot := a.seedSlot(day(19, 9, 30), 1), a.seedSlot(day(26, 10, 0), 1)
	appt := jsonBody[apptJSON](t, a.book(tok1, oldSlot))
	require.Equal(t, http.StatusCreated, a.book(a.tokenFor(p2.ID, domain.RolePatient), fullSlot).Code)

	rec := a.call(http.MethodPatch, "/api/v1/appointments/"+appt.ID, map[string]any{"slot_id": fullSlot.String()}, tok1)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "slot_full", errCode(t, rec))
	assert.Equal(t, 1, a.bookedCount(oldSlot), "元の予約は維持")
	assert.Equal(t, 1, a.bookedCount(fullSlot))
	list := jsonBody[[]apptJSON](t, a.call(http.MethodGet, "/api/v1/appointments/me", nil, tok1))
	assert.Equal(t, "2026-10-19T09:30:00+09:00", list[0].SlotStart)
}

func TestAppointments_RescheduleValidation(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	tok := a.tokenFor(p.ID, domain.RolePatient)
	slot := a.seedSlot(day(19, 9, 30), 1)
	appt := jsonBody[apptJSON](t, a.book(tok, slot))
	path := "/api/v1/appointments/" + appt.ID

	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPatch, path, map[string]any{"slot_id": slot.String()}, tok).Code, "同じ枠")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPatch, path, map[string]any{"slot_id": uuid.NewString()}, tok).Code, "存在しない枠")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPatch, path, map[string]any{}, tok).Code, "slot_id なし")
	assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPatch, path, map[string]any{"slot_id": "x"}, tok).Code, "UUID でない")
}
