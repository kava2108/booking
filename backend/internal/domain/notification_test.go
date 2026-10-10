package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IMP 12 / 14: 通知 payload には要配慮個人情報（症状メモ等）を含めない（REQ-007-AC-4）

func TestAppointmentNotice_PayloadKeys(t *testing.T) {
	prev := time.Date(2026, 10, 12, 9, 0, 0, 0, JST)
	b, err := json.Marshal(AppointmentNotice{
		AppointmentID: "id-1", AppointmentNo: "A-20261019-0042", StartAt: time.Date(2026, 10, 19, 9, 30, 0, 0, JST),
		Department: "内科", Doctor: "山田", PreviousStartAt: &prev,
	})
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.ElementsMatch(t, []string{"appointment_id", "appointment_no", "start_at", "department", "doctor", "previous_start_at"}, keys(m))
	assert.Equal(t, "2026-10-19T09:30:00+09:00", m["start_at"], "JST で保存")
}

func TestAppointmentNotice_OmitsPreviousStartWhenNil(t *testing.T) {
	b, _ := json.Marshal(AppointmentNotice{AppointmentNo: "A-1"})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	assert.NotContains(t, m, "previous_start_at")
}

func TestNotificationKinds(t *testing.T) {
	assert.Equal(t, "appointment_confirmed", KindAppointmentConfirmed)
	assert.Equal(t, "appointment_canceled", KindAppointmentCanceled)
	assert.Equal(t, "appointment_rescheduled", KindAppointmentRescheduled)
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
