package app_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 11-clinic-reservation-slot-api-admin-settings TASK-0001: 公開カタログと管理者 API の統合テスト

type slotJSON struct {
	ID                string `json:"id"`
	DoctorID          int64  `json:"doctor_id"`
	StartAt           string `json:"start_at"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason"`
}

func (a *clinicApp) putMondaySchedule(token string) {
	a.t.Helper()
	rec := a.call(http.MethodPut, "/api/v1/admin/schedules", map[string]any{
		"doctor_id": a.doctorID,
		"schedules": []map[string]any{{"weekday": 1, "start_time": "09:00", "end_time": "12:00", "slot_minutes": 30, "capacity": 2}},
	}, token)
	require.Equal(a.t, http.StatusOK, rec.Code, rec.Body.String())
}

func (a *clinicApp) slots(query string) []slotJSON {
	a.t.Helper()
	rec := a.call(http.MethodGet, "/api/v1/slots?"+query, nil, "")
	require.Equal(a.t, http.StatusOK, rec.Code, rec.Body.String())
	return jsonBody[[]slotJSON](a.t, rec)
}

// ---------- 公開カタログ (REQ-001) ----------

func TestCatalog_PublicWithoutToken(t *testing.T) {
	a := newClinicApp(t, nil)

	rec := a.call(http.MethodGet, "/api/v1/departments", nil, "")
	require.Equal(t, http.StatusOK, rec.Code)
	deps := jsonBody[[]map[string]any](t, rec)
	require.Len(t, deps, 1)
	assert.Equal(t, "内科", deps[0]["name"])
	assert.NotContains(t, deps[0], "standard_consult_minutes", "診療科の内部設定は公開しない")

	rec = a.call(http.MethodGet, fmt.Sprintf("/api/v1/doctors?department_id=%d", a.deptID), nil, "")
	require.Equal(t, http.StatusOK, rec.Code)
	docs := jsonBody[[]map[string]any](t, rec)
	require.Len(t, docs, 1)
	assert.Equal(t, "山田", docs[0]["name"])
	assert.NotContains(t, docs[0], "active")
}

func TestCatalog_InvalidParams(t *testing.T) {
	a := newClinicApp(t, nil)
	for name, path := range map[string]string{
		"医師: department_id なし": "/api/v1/doctors",
		"医師: 数値でない":            "/api/v1/doctors?department_id=abc",
		"医師: 0":                "/api/v1/doctors?department_id=0",
		"枠: 日付なし":              fmt.Sprintf("/api/v1/slots?department_id=%d", a.deptID),
		"枠: 日付が不正":             fmt.Sprintf("/api/v1/slots?department_id=%d&date=2026-02-30", a.deptID),
		"枠: 日付の書式":             fmt.Sprintf("/api/v1/slots?department_id=%d&date=10/12/2026", a.deptID),
		"枠: 診療科なし":             "/api/v1/slots?date=2026-10-12",
		"枠: 存在しない診療科":          "/api/v1/slots?department_id=9999&date=2026-10-12",
		"枠: doctor_id が不正":     fmt.Sprintf("/api/v1/slots?department_id=%d&date=2026-10-12&doctor_id=x", a.deptID),
	} {
		t.Run(name, func(t *testing.T) {
			rec := a.call(http.MethodGet, path, nil, "")
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			assert.Equal(t, "validation_failed", jsonBody[map[string]any](t, rec)["error"])
		})
	}
}

func TestCatalog_PublicRateLimit(t *testing.T) {
	a := newClinicApp(t, func(c *config.Config) { c.RateLimitPublicMax = 3 })

	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusOK, a.call(http.MethodGet, "/api/v1/departments", nil, "").Code)
	}
	rec := a.call(http.MethodGet, "/api/v1/departments", nil, "")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// ---------- スケジュールから枠を生成 (REQ-006-AC-1, REQ-001) ----------

func TestAdmin_SchedulesGenerateSlots(t *testing.T) {
	a := newClinicApp(t, nil)
	a.putMondaySchedule(a.token(domain.RoleClinicAdmin))

	got := a.slots(fmt.Sprintf("department_id=%d&date=2026-10-12", a.deptID)) // 月曜（今日）
	require.Len(t, got, 6, "9:00〜12:00 を 30 分 = 6 枠")
	assert.Equal(t, "2026-10-12T09:00:00+09:00", got[0].StartAt)
	assert.Equal(t, "2026-10-12T11:30:00+09:00", got[5].StartAt)
	assert.False(t, got[0].Available, "現在 9:00 ちょうどの枠は締切後")
	assert.Equal(t, "past_cutoff", got[0].UnavailableReason)
	assert.True(t, got[1].Available, "9:30 は予約可")
	assert.Empty(t, got[1].UnavailableReason)

	next := a.slots(fmt.Sprintf("department_id=%d&date=2026-10-19", a.deptID)) // 来週月曜
	require.Len(t, next, 6)
	assert.True(t, next[0].Available)

	assert.Empty(t, a.slots(fmt.Sprintf("department_id=%d&date=2026-10-13", a.deptID)), "火曜は診療なし → 空配列")
}

// 公開 API の枠に内部情報（定員・予約数）を含めない
func TestSlots_DoNotExposeInternalCounts(t *testing.T) {
	a := newClinicApp(t, nil)
	a.putMondaySchedule(a.token(domain.RoleClinicAdmin))

	rec := a.call(http.MethodGet, fmt.Sprintf("/api/v1/slots?department_id=%d&date=2026-10-19", a.deptID), nil, "")
	for _, s := range jsonBody[[]map[string]any](t, rec) {
		assert.NotContains(t, s, "capacity")
		assert.NotContains(t, s, "booked_count")
		assert.NotContains(t, s, "status")
	}
}

// 同じスケジュールを何度保存しても枠は増えない（冪等）
func TestAdmin_SchedulesAreIdempotent(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)

	a.putMondaySchedule(admin)
	a.putMondaySchedule(admin)

	var n int
	require.NoError(t, a.pool.QueryRow(t.Context(), "SELECT count(*) FROM slots").Scan(&n))
	assert.Equal(t, 6*5, n, "今日〜30 日後に月曜が 5 回（10/12, 19, 26, 11/2, 9）× 6 枠")
}

// 医師指名で絞り込める
func TestSlots_FilterByDoctor(t *testing.T) {
	a := newClinicApp(t, nil)
	a.putMondaySchedule(a.token(domain.RoleClinicAdmin))
	var other int64
	require.NoError(t, a.pool.QueryRow(t.Context(), `INSERT INTO doctors(department_id,name) VALUES ($1,'佐藤') RETURNING id`, a.deptID).Scan(&other))

	assert.Len(t, a.slots(fmt.Sprintf("department_id=%d&date=2026-10-19&doctor_id=%d", a.deptID, a.doctorID)), 6)
	assert.Empty(t, a.slots(fmt.Sprintf("department_id=%d&date=2026-10-19&doctor_id=%d", a.deptID, other)))
}

// スケジュールの再保存では、予約済みの枠（booked_count>0）を壊さない
func TestAdmin_ResavingSchedulesKeepsBookedSlots(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)
	a.putMondaySchedule(admin)
	_, err := a.pool.Exec(t.Context(), `UPDATE slots SET booked_count = 1 WHERE start_at = '2026-10-19 10:00+09'`)
	require.NoError(t, err)

	a.putMondaySchedule(admin)

	var booked int
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT booked_count FROM slots WHERE start_at = '2026-10-19 10:00+09'`).Scan(&booked))
	assert.Equal(t, 1, booked)
}

// 休診期間に生成した枠は closed（再保存・日次ジョブでも休診が復活しない）
func TestAdmin_GeneratedSlotsRespectExistingClosures(t *testing.T) {
	a := newClinicApp(t, nil)
	_, err := a.pool.Exec(t.Context(), `INSERT INTO closures(doctor_id, period, reason) VALUES ($1, tstzrange('2026-10-19 00:00+09','2026-10-20 00:00+09'), '学会')`, a.doctorID)
	require.NoError(t, err)

	a.putMondaySchedule(a.token(domain.RoleClinicAdmin))

	for _, s := range a.slots(fmt.Sprintf("department_id=%d&date=2026-10-19", a.deptID)) {
		assert.False(t, s.Available)
		assert.Equal(t, "closed", s.UnavailableReason)
	}
	assert.True(t, a.slots(fmt.Sprintf("department_id=%d&date=2026-10-26", a.deptID))[0].Available, "休診外の週は予約可")
}

// ---------- 認可 (REQ-005-AC-4 の管理者版) ----------

func TestAdminEndpoints_Authorization(t *testing.T) {
	a := newClinicApp(t, nil)
	body := map[string]any{"doctor_id": a.doctorID, "schedules": []any{}}

	for _, path := range []string{"/api/v1/admin/schedules", "/api/v1/admin/settings"} {
		b := any(body)
		if path == "/api/v1/admin/settings" {
			b = map[string]any{"cancel_deadline_minutes": 30}
		}
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, a.call(http.MethodPut, path, b, "").Code, "トークンなし")
			assert.Equal(t, http.StatusUnauthorized, a.call(http.MethodPut, path, b, "garbage").Code, "不正トークン")
			assert.Equal(t, http.StatusForbidden, a.call(http.MethodPut, path, b, a.token(domain.RolePatient)).Code, "患者")
			assert.Equal(t, http.StatusForbidden, a.call(http.MethodPut, path, b, a.token(domain.RoleStaff)).Code, "受付スタッフ")
			assert.Equal(t, http.StatusForbidden, a.call(http.MethodPut, path, b, a.token(domain.RoleSystemAdmin)).Code, "システム管理者")
			assert.Equal(t, http.StatusOK, a.call(http.MethodPut, path, b, a.token(domain.RoleClinicAdmin)).Code, "クリニック管理者")
		})
	}
}

// ---------- 入力検証 ----------

func TestAdmin_SchedulesValidation(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)
	put := func(body any) (int, map[string]any) {
		rec := a.call(http.MethodPut, "/api/v1/admin/schedules", body, admin)
		return rec.Code, jsonBody[map[string]any](t, rec)
	}
	fields := func(m map[string]any) []string {
		var out []string
		for _, d := range m["details"].([]any) {
			out = append(out, d.(map[string]any)["field"].(string))
		}
		return out
	}
	sched := func(s map[string]any) map[string]any {
		return map[string]any{"doctor_id": a.doctorID, "schedules": []any{s}}
	}
	ok := map[string]any{"weekday": 1, "start_time": "09:00", "end_time": "12:00", "slot_minutes": 30, "capacity": 1}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range ok {
			m[kk] = vv
		}
		m[k] = v
		return m
	}

	t.Run("終了が開始より前", func(t *testing.T) {
		code, body := put(sched(with("end_time", "08:00")))
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, fields(body), "schedules[0].end_time")
	})
	t.Run("曜日が範囲外", func(t *testing.T) {
		code, body := put(sched(with("weekday", 9)))
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, fields(body), "schedules[0].weekday")
	})
	t.Run("時刻の書式", func(t *testing.T) {
		code, body := put(sched(with("start_time", "9am")))
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, fields(body), "schedules[0].start_time")
	})
	t.Run("存在しない医師", func(t *testing.T) {
		code, body := put(map[string]any{"doctor_id": 99999, "schedules": []any{}})
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, fields(body), "doctor_id")
	})
	t.Run("doctor_id なし", func(t *testing.T) {
		code, _ := put(map[string]any{"schedules": []any{}})
		assert.Equal(t, http.StatusUnprocessableEntity, code)
	})
	t.Run("壊れた JSON", func(t *testing.T) {
		rec := a.call(http.MethodPut, "/api/v1/admin/schedules", "{oops", admin)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
	t.Run("検証エラーのとき既存スケジュールは変わらない", func(t *testing.T) {
		a.putMondaySchedule(admin)
		put(sched(with("end_time", "08:00")))

		var n int
		require.NoError(t, a.pool.QueryRow(t.Context(), "SELECT count(*) FROM doctor_schedules").Scan(&n))
		assert.Equal(t, 1, n)
	})
}

// ---------- 設定 (REQ-006-AC-2) ----------

func TestAdmin_SettingsAffectBookingWindow(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)
	a.putMondaySchedule(admin)

	rec := a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{"booking_window_days": 5, "cancel_deadline_minutes": 120}, admin)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := jsonBody[map[string]int](t, rec)
	assert.Equal(t, 5, got["booking_window_days"])
	assert.Equal(t, 120, got["cancel_deadline_minutes"])

	// 12 日 + 5 日 = 17 日が最終日。19 日の枠は受付期間外
	next := a.slots(fmt.Sprintf("department_id=%d&date=2026-10-19", a.deptID))
	require.NotEmpty(t, next)
	assert.False(t, next[0].Available)
	assert.Equal(t, "outside_window", next[0].UnavailableReason)

	// 部分更新: 受付期間だけ変えてもキャンセル期限は維持
	rec = a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{"booking_window_days": 30}, admin)
	got = jsonBody[map[string]int](t, rec)
	assert.Equal(t, 120, got["cancel_deadline_minutes"])
}

func TestAdmin_SettingsDepartmentFieldsAndValidation(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)

	rec := a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{
		"departments": []map[string]any{{"id": a.deptID, "standard_consult_minutes": 15, "walkin_cutoff": "11:30", "walkin_capacity": 20}},
	}, admin)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var std *int32
	var cutoff string
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT standard_consult_minutes, walkin_cutoff::text FROM departments WHERE id=$1`, a.deptID).Scan(&std, &cutoff))
	require.NotNil(t, std)
	assert.EqualValues(t, 15, *std)
	assert.Equal(t, "11:30:00", cutoff)

	for name, body := range map[string]map[string]any{
		"受付期間 0":    {"booking_window_days": 0},
		"キャンセル期限が負": {"cancel_deadline_minutes": -5},
		"未知の診療科":    {"departments": []map[string]any{{"id": 99999}}},
		"締切時刻の書式":   {"departments": []map[string]any{{"id": a.deptID, "walkin_cutoff": "noon"}}},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusUnprocessableEntity, a.call(http.MethodPut, "/api/v1/admin/settings", body, admin).Code)
		})
	}
}

// 未知の診療科を含む更新は全体がロールバックされる（医療機関設定だけ更新されない）
func TestAdmin_SettingsAreAtomic(t *testing.T) {
	a := newClinicApp(t, nil)
	admin := a.token(domain.RoleClinicAdmin)

	rec := a.call(http.MethodPut, "/api/v1/admin/settings", map[string]any{
		"cancel_deadline_minutes": 999, "departments": []map[string]any{{"id": 99999}},
	}, admin)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var minutes int
	require.NoError(t, a.pool.QueryRow(t.Context(), `SELECT cancel_deadline_minutes FROM clinic_settings`).Scan(&minutes))
	assert.Equal(t, 60, minutes, "既定値のまま")
}
