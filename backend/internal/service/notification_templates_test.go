package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 14-clinic-reservation-notification-outbox TASK-0001: メール本文の描画（REQ-007-AC-4: 症状・診断を含めない）

func noticePayload(t *testing.T, mutate func(*domain.AppointmentNotice)) []byte {
	t.Helper()
	n := domain.AppointmentNotice{
		AppointmentID: "id-1", AppointmentNo: "A-20261019-0042", StartAt: time.Date(2026, 10, 19, 9, 30, 0, 0, domain.JST),
		Department: "内科", Doctor: "山田",
	}
	if mutate != nil {
		mutate(&n)
	}
	b, err := json.Marshal(n)
	require.NoError(t, err)
	return b
}

func TestRender_AppointmentConfirmed(t *testing.T) {
	m, err := service.RenderNotification(domain.KindAppointmentConfirmed, "山田 太郎", noticePayload(t, nil))
	require.NoError(t, err)

	assert.Contains(t, m.Subject, "予約")
	for _, want := range []string{"山田 太郎 様", "A-20261019-0042", "2026年10月19日(月) 09:30", "内科", "山田"} {
		assert.Contains(t, m.Body, want)
	}
}

// UTC で保存されていても JST で表示する（患者に 9 時間ずれた時刻を見せない）
func TestRender_TimesAreShownInJST(t *testing.T) {
	m, err := service.RenderNotification(domain.KindAppointmentConfirmed, "n", noticePayload(t, func(n *domain.AppointmentNotice) {
		n.StartAt = time.Date(2026, 10, 19, 0, 30, 0, 0, time.UTC) // = JST 09:30
	}))
	require.NoError(t, err)
	assert.Contains(t, m.Body, "09:30")
	assert.NotContains(t, m.Body, "00:30")
}

func TestRender_AppointmentCanceled(t *testing.T) {
	m, err := service.RenderNotification(domain.KindAppointmentCanceled, "n", noticePayload(t, nil))
	require.NoError(t, err)
	assert.Contains(t, m.Subject, "キャンセル")
	assert.Contains(t, m.Body, "A-20261019-0042")
	assert.Contains(t, m.Body, "2026年10月19日(月) 09:30")
}

func TestRender_AppointmentRescheduledShowsBothTimes(t *testing.T) {
	prev := time.Date(2026, 10, 12, 14, 0, 0, 0, domain.JST)
	m, err := service.RenderNotification(domain.KindAppointmentRescheduled, "n", noticePayload(t, func(n *domain.AppointmentNotice) { n.PreviousStartAt = &prev }))
	require.NoError(t, err)
	assert.Contains(t, m.Subject, "変更")
	assert.Contains(t, m.Body, "2026年10月12日(月) 14:00", "変更前")
	assert.Contains(t, m.Body, "2026年10月19日(月) 09:30", "変更後")
}

func TestRender_Reminder(t *testing.T) {
	m, err := service.RenderNotification(domain.KindAppointmentReminder, "n", noticePayload(t, nil))
	require.NoError(t, err)
	assert.Contains(t, m.Subject, "明日")
	assert.Contains(t, m.Body, "2026年10月19日(月) 09:30")
	assert.Contains(t, m.Body, "内科")
}

func TestRender_QueueNear(t *testing.T) {
	b, _ := json.Marshal(domain.QueueNearNotice{QueueEntryID: "q1", TicketNo: 12, Department: "内科", AheadCount: 3})
	m, err := service.RenderNotification(domain.KindQueueNear, "山田 太郎", b)
	require.NoError(t, err)
	assert.Contains(t, m.Subject, "順番")
	assert.Contains(t, m.Body, "あと3人")
	assert.Contains(t, m.Body, "12")
	assert.Contains(t, m.Body, "内科")

	b, _ = json.Marshal(domain.QueueNearNotice{TicketNo: 5, Department: "内科", AheadCount: 0})
	m, err = service.RenderNotification(domain.KindQueueNear, "n", b)
	require.NoError(t, err)
	assert.Contains(t, m.Body, "次", "あと 0 人は「次にお呼びします」")
	assert.NotContains(t, m.Body, "あと0人")
}

// REQ-007-AC-4: payload に症状メモ等が紛れ込んでも本文に出ない（既知のフィールドだけを描画する）
func TestRender_NeverIncludesSymptomOrUnknownFields(t *testing.T) {
	for _, kind := range []string{domain.KindAppointmentConfirmed, domain.KindAppointmentCanceled, domain.KindAppointmentRescheduled, domain.KindAppointmentReminder} {
		t.Run(kind, func(t *testing.T) {
			var m map[string]any
			require.NoError(t, json.Unmarshal(noticePayload(t, nil), &m))
			m["symptom_note"] = "糖尿病の疑い"
			m["diagnosis"] = "要精密検査"
			m["email"] = "secret@example.com"
			b, _ := json.Marshal(m)

			out, err := service.RenderNotification(kind, "n", b)
			require.NoError(t, err)
			for _, leaked := range []string{"糖尿病", "精密検査", "secret@example.com", "symptom", "diagnosis"} {
				assert.NotContains(t, out.Subject+out.Body, leaked)
			}
		})
	}
}

// 名前に改行があっても本文の体裁・ヘッダーを壊さない
func TestRender_NameIsSanitized(t *testing.T) {
	m, err := service.RenderNotification(domain.KindAppointmentConfirmed, "山田\r\nBcc: evil@example.com", noticePayload(t, nil))
	require.NoError(t, err)
	assert.NotContains(t, m.Body, "山田\r\nBcc")
	assert.NotContains(t, m.Subject, "\n")
	assert.NotContains(t, m.Subject, "\r")
}

func TestRender_Errors(t *testing.T) {
	_, err := service.RenderNotification("something_else", "n", []byte(`{}`))
	assert.ErrorIs(t, err, service.ErrUnknownNotificationKind)

	_, err = service.RenderNotification(domain.KindAppointmentConfirmed, "n", []byte(`not json`))
	assert.Error(t, err)
	assert.NotErrorIs(t, err, service.ErrUnknownNotificationKind)

	_, err = service.RenderNotification(domain.KindAppointmentConfirmed, "n", []byte(`{}`))
	assert.Error(t, err, "必須項目（予約番号・日時）が無い payload は送らない")
}

func TestRender_EmptyNameFallsBack(t *testing.T) {
	m, err := service.RenderNotification(domain.KindAppointmentConfirmed, "", noticePayload(t, nil))
	require.NoError(t, err)
	assert.Contains(t, m.Body, "患者様")
}
