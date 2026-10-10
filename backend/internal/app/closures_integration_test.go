package app_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 16-clinic-reservation-closures TASK-0001: 休診設定の統合テスト（REQ-005-AC-3）

type closureResultJSON struct {
	ClosedSlots int `json:"closed_slots"`
	Notified    int `json:"notified"`
}

func rfc(t time.Time) string { return t.Format(time.RFC3339) }

func (a *clinicApp) closure(tok string, body map[string]any) *httptest.ResponseRecorder {
	return a.call(http.MethodPost, "/api/v1/staff/closures", body, tok)
}

func (a *clinicApp) slotStatus(id uuid.UUID) string {
	var s string
	require.NoError(a.t, a.pool.QueryRow(a.t.Context(), `SELECT status FROM slots WHERE id=$1`, id).Scan(&s))
	return s
}

func TestClosure_ClosesSlotsAndNotifiesBookedPatients(t *testing.T) {
	a := newClinicApp(t, nil)
	staff := a.staffTok()
	p1, p2, p3 := a.newUser("p1@example.com", domain.RolePatient), a.newUser("p2@example.com", domain.RolePatient), a.newUser("p3@example.com", domain.RolePatient)
	in1, in2 := a.seedSlot(day(13, 9, 30), 2), a.seedSlot(day(13, 11, 0), 2)
	atEnd := a.seedSlot(day(13, 12, 0), 2) // 終了ちょうど = 範囲外
	other := a.seedSlot(day(14, 9, 30), 2) // 別の日
	free := a.seedSlot(day(13, 10, 0), 2)  // 予約なしの枠も閉じる
	a.book(a.tokenFor(p1.ID, domain.RolePatient), in1)
	a.book(a.tokenFor(p2.ID, domain.RolePatient), in2)
	a.book(a.tokenFor(p3.ID, domain.RolePatient), in1)
	a.book(a.tokenFor(p1.ID, domain.RolePatient), other)
	a.book(a.tokenFor(p3.ID, domain.RolePatient), atEnd)
	canceled := jsonBody[apptJSON](t, a.book(a.tokenFor(p2.ID, domain.RolePatient), free))
	require.Equal(t, http.StatusNoContent, a.call(http.MethodDelete, "/api/v1/appointments/"+canceled.ID, nil, a.tokenFor(p2.ID, domain.RolePatient)).Code)
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT count(*) FROM notifications`).Scan(new(int)))
	before := a.count(`SELECT count(*) FROM notifications`)

	rec := a.closure(staff, map[string]any{"doctor_id": a.doctorID, "from_at": rfc(day(13, 9, 0)), "to_at": rfc(day(13, 12, 0)), "reason": "学会"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	res := jsonBody[closureResultJSON](t, rec)
	assert.Equal(t, 3, res.ClosedSlots, "9:30 / 10:00 / 11:00 の枠")
	assert.Equal(t, 3, res.Notified, "予約者 3 人（取消済みは除く）")
	assert.Equal(t, "closed", a.slotStatus(in1))
	assert.Equal(t, "closed", a.slotStatus(in2))
	assert.Equal(t, "closed", a.slotStatus(free))
	assert.Equal(t, "open", a.slotStatus(atEnd), "範囲は [from, to)")
	assert.Equal(t, "open", a.slotStatus(other))

	assert.Equal(t, 3, a.count(`SELECT count(*) FROM notifications WHERE kind='clinic_closure'`)-0)
	assert.Equal(t, before+3, a.count(`SELECT count(*) FROM notifications`))
	var payload string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT payload::text FROM notifications WHERE kind='clinic_closure' LIMIT 1`).Scan(&payload))
	assert.NotContains(t, payload, "学会", "理由の自由記述は通知に載せない")
	assert.NotContains(t, payload, "咳", "症状メモも載せない")

	// 予約は自動キャンセルしない（要件どおり通知のみ）
	assert.Equal(t, 3, a.count(`SELECT count(*) FROM appointments WHERE slot_id = ANY($1) AND status='booked'`, []uuid.UUID{in1, in2}))

	// 操作は監査ログに記録される
	var action, target string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT action, target FROM audit_logs ORDER BY id DESC LIMIT 1`).Scan(&action, &target))
	assert.Equal(t, "create_closure", action)
	assert.Equal(t, fmt.Sprintf("doctor:%d", a.doctorID), target)
}

// 休診した枠は予約できず、検索では closed と表示される
func TestClosure_ClosedSlotsCannotBeBooked(t *testing.T) {
	a := newClinicApp(t, nil)
	slot := a.seedSlot(day(13, 9, 30), 2)
	require.Equal(t, http.StatusCreated, a.closure(a.staffTok(), map[string]any{"doctor_id": a.doctorID, "from_at": rfc(day(13, 0, 0)), "to_at": rfc(day(14, 0, 0))}).Code)

	p := a.newUser("p@example.com", domain.RolePatient)
	rec := a.book(a.tokenFor(p.ID, domain.RolePatient), slot)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	got := a.slots(fmt.Sprintf("department_id=%d&date=2026-10-13", a.deptID))
	require.Len(t, got, 1)
	assert.Equal(t, "closed", got[0].UnavailableReason)
}

// 休診中は待ち時間が「算出不可(closed)」になる
func TestClosure_WaitTimeBecomesUnavailable(t *testing.T) {
	a := newClinicApp(t, nil)
	a.openClinic(10)
	w, _ := a.waitTimes()
	require.Equal(t, "available", w.Departments[0].Status)

	require.Equal(t, http.StatusCreated, a.closure(a.staffTok(), map[string]any{"doctor_id": a.doctorID, "from_at": rfc(day(12, 8, 0)), "to_at": rfc(day(12, 12, 0))}).Code)
	*a.clock = clinicNow.Add(11 * time.Second) // キャッシュ期限を過ぎてから

	w, _ = a.waitTimes()
	assert.Equal(t, "unavailable", w.Departments[0].Status)
	assert.Equal(t, "closed", w.Departments[0].Reason)
}

// 医師を指定しない休診は、全医師の枠が対象
func TestClosure_WholeClinic(t *testing.T) {
	a := newClinicApp(t, nil)
	var other int64
	require.NoError(t, a.pool.QueryRow(t.Context(), `INSERT INTO doctors(department_id,name) VALUES ($1,'佐藤') RETURNING id`, a.deptID).Scan(&other))
	var otherSlot uuid.UUID
	require.NoError(t, a.pool.QueryRow(t.Context(), `INSERT INTO slots(department_id,doctor_id,start_at,capacity) VALUES ($1,$2,$3,1) RETURNING id`, a.deptID, other, day(13, 9, 30)).Scan(&otherSlot))
	mine := a.seedSlot(day(13, 10, 0), 1)

	rec := a.closure(a.staffTok(), map[string]any{"from_at": rfc(day(13, 0, 0)), "to_at": rfc(day(14, 0, 0))})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	assert.Equal(t, 2, jsonBody[closureResultJSON](t, rec).ClosedSlots)
	assert.Equal(t, "closed", a.slotStatus(otherSlot))
	assert.Equal(t, "closed", a.slotStatus(mine))
	assert.Equal(t, "clinic", func() string {
		var s string
		require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT target FROM audit_logs ORDER BY id DESC LIMIT 1`).Scan(&s))
		return s
	}())
}

// 同じ休診を繰り返しても、同じ予約者に二重に通知しない
func TestClosure_RepeatedRequestDoesNotNotifyTwice(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	a.book(a.tokenFor(p.ID, domain.RolePatient), a.seedSlot(day(13, 9, 30), 1))
	body := map[string]any{"doctor_id": a.doctorID, "from_at": rfc(day(13, 0, 0)), "to_at": rfc(day(14, 0, 0))}
	staff := a.staffTok()

	first := jsonBody[closureResultJSON](t, a.closure(staff, body))
	second := jsonBody[closureResultJSON](t, a.closure(staff, body))

	assert.Equal(t, closureResultJSON{ClosedSlots: 1, Notified: 1}, first)
	assert.Equal(t, closureResultJSON{}, second)
	assert.Equal(t, 1, a.count(`SELECT count(*) FROM notifications WHERE kind='clinic_closure'`))
}

func TestClosure_Validation(t *testing.T) {
	a := newClinicApp(t, nil)
	staff := a.staffTok()
	cases := map[string]map[string]any{
		"開始が終了以降":      {"from_at": rfc(day(13, 12, 0)), "to_at": rfc(day(13, 9, 0))},
		"終了が過去":        {"from_at": rfc(day(10, 9, 0)), "to_at": rfc(day(11, 9, 0))},
		"未知の医師":        {"doctor_id": 99999, "from_at": rfc(day(13, 9, 0)), "to_at": rfc(day(13, 12, 0))},
		"from_at なし":   {"to_at": rfc(day(13, 12, 0))},
		"to_at なし":     {"from_at": rfc(day(13, 9, 0))},
		"理由が長すぎる":      {"from_at": rfc(day(13, 9, 0)), "to_at": rfc(day(13, 12, 0)), "reason": strings.Repeat("あ", 201)},
		"期間が長すぎる":      {"from_at": rfc(day(13, 9, 0)), "to_at": rfc(day(13, 9, 0).AddDate(0, 0, 400))},
		"doctor_id が負": {"doctor_id": -1, "from_at": rfc(day(13, 9, 0)), "to_at": rfc(day(13, 12, 0))},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := a.closure(staff, body)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
		})
	}
	assert.Equal(t, http.StatusBadRequest, a.call(http.MethodPost, "/api/v1/staff/closures", map[string]any{"from_at": "not-a-time", "to_at": "x"}, staff).Code, "日時の書式不正")
	assert.Equal(t, http.StatusBadRequest, a.call(http.MethodPost, "/api/v1/staff/closures", "{oops", staff).Code)
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM closures`), "検証エラーでは何も作らない")
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM audit_logs`))
}

// 途中で失敗（監査ログの actor が存在しない → FK 違反）したら、休診も枠の変更も通知も全て巻き戻る
func TestClosure_IsAtomic(t *testing.T) {
	a := newClinicApp(t, nil)
	p := a.newUser("p@example.com", domain.RolePatient)
	slot := a.seedSlot(day(13, 9, 30), 1)
	a.book(a.tokenFor(p.ID, domain.RolePatient), slot)
	ghost := a.tokenFor(uuid.New(), domain.RoleStaff) // JWT は有効だが users に存在しない

	rec := a.closure(ghost, map[string]any{"doctor_id": a.doctorID, "from_at": rfc(day(13, 0, 0)), "to_at": rfc(day(14, 0, 0))})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM closures`))
	assert.Equal(t, "open", a.slotStatus(slot))
	assert.Equal(t, 0, a.count(`SELECT count(*) FROM notifications WHERE kind='clinic_closure'`))
}

func TestClosure_Authorization(t *testing.T) {
	a := newClinicApp(t, nil)
	body := map[string]any{"from_at": rfc(day(13, 9, 0)), "to_at": rfc(day(13, 12, 0))}

	assert.Equal(t, http.StatusUnauthorized, a.closure("", body).Code)
	assert.Equal(t, http.StatusForbidden, a.closure(a.token(domain.RolePatient), body).Code)
	assert.Equal(t, http.StatusForbidden, a.closure(a.token(domain.RoleSystemAdmin), body).Code)
	assert.Equal(t, http.StatusCreated, a.closure(a.staffTok(), body).Code)
	admin := a.newUser("admin@example.com", domain.RoleClinicAdmin)
	assert.Equal(t, http.StatusCreated, a.closure(a.tokenFor(admin.ID, domain.RoleClinicAdmin), body).Code)
}

// 休診設定後に枠を再生成しても、休診が復活しない
func TestClosure_SurvivesSlotRegeneration(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)
	require.Equal(t, http.StatusCreated, a.closure(a.staffTok(), map[string]any{"doctor_id": a.doctorID, "from_at": rfc(day(19, 0, 0)), "to_at": rfc(day(20, 0, 0))}).Code)

	a.putMondaySchedule(admin) // 10/19(月) の枠が生成される

	for _, s := range a.slots(fmt.Sprintf("department_id=%d&date=2026-10-19", a.deptID)) {
		assert.Equal(t, "closed", s.UnavailableReason)
	}
}
