package domain

import "time"

// 通知の種別（notifications.kind）。
const (
	KindAppointmentConfirmed   = "appointment_confirmed"
	KindAppointmentCanceled    = "appointment_canceled"
	KindAppointmentRescheduled = "appointment_rescheduled"
)

// AppointmentNotice は予約関連通知の payload。
// 要配慮個人情報（症状メモなど）は意図的にフィールドを持たない。通知本文・DB の payload に症状が入らないことを型で保証する。
type AppointmentNotice struct {
	AppointmentID   string     `json:"appointment_id"`
	AppointmentNo   string     `json:"appointment_no"`
	StartAt         time.Time  `json:"start_at"`
	Department      string     `json:"department"`
	Doctor          string     `json:"doctor"`
	PreviousStartAt *time.Time `json:"previous_start_at,omitempty"` // 日時変更のときのみ
}

// リマインドと順番接近の通知種別。いずれも 1 件につき 1 回だけ送る（DB の部分ユニークインデックスで保証）。
const (
	KindAppointmentReminder = "appointment_reminder" // 前日 18:00(JST) 以降、翌日の予約に 1 回
	KindQueueNear           = "queue_near"           // 前にいる人数が閾値以下になったときに 1 回
)

// QueueNearNotice は「あと N 人」通知の payload。氏名・症状は含めない。
type QueueNearNotice struct {
	QueueEntryID string `json:"queue_entry_id"` // 冪等キー
	TicketNo     int    `json:"ticket_no"`
	Department   string `json:"department"`
	AheadCount   int    `json:"ahead_count"`
}
