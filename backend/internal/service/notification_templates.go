package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kava2108/booking/backend/internal/domain"
)

// ErrUnknownNotificationKind は描画できない通知種別。再試行しても無駄なので failed にする。
var ErrUnknownNotificationKind = errors.New("unknown notification kind")

// RenderedMail は送信するメールの件名と本文。
type RenderedMail struct{ Subject, Body string }

var weekdayJP = [...]string{"日", "月", "火", "水", "木", "金", "土"}

// formatJST は "2026年10月19日(月) 09:30"（JST）。保存形式(UTC 等)に関わらず診療所の時刻で表示する。
func formatJST(t time.Time) string {
	j := t.In(domain.JST)
	return fmt.Sprintf("%d年%d月%d日(%s) %02d:%02d", j.Year(), int(j.Month()), j.Day(), weekdayJP[j.Weekday()], j.Hour(), j.Minute())
}

// RenderNotification は通知の種別と payload からメールを組み立てる。
// payload は既知の型にデコードして必要な項目だけを使う。未知のキー(症状メモなど)が紛れ込んでも本文には出ない。
// IMP 14-clinic-reservation-notification-outbox TASK-0001 / REQ-007-AC-4
func RenderNotification(kind, recipientName string, payload []byte) (RenderedMail, error) {
	greeting := "患者様"
	if n := strings.TrimSpace(sanitizeText(recipientName)); n != "" {
		greeting = n + " 様"
	}

	switch kind {
	case domain.KindAppointmentConfirmed, domain.KindAppointmentCanceled, domain.KindAppointmentRescheduled, domain.KindAppointmentReminder, domain.KindClinicClosure:
		var n domain.AppointmentNotice
		if err := json.Unmarshal(payload, &n); err != nil {
			return RenderedMail{}, fmt.Errorf("decode payload: %w", err)
		}
		if n.AppointmentNo == "" || n.StartAt.IsZero() {
			return RenderedMail{}, errors.New("invalid payload: appointment_no and start_at are required")
		}
		return renderAppointment(kind, greeting, n), nil
	case domain.KindQueueNear:
		var n domain.QueueNearNotice
		if err := json.Unmarshal(payload, &n); err != nil {
			return RenderedMail{}, fmt.Errorf("decode payload: %w", err)
		}
		if n.Department == "" || n.TicketNo <= 0 {
			return RenderedMail{}, errors.New("invalid payload: department and ticket_no are required")
		}
		return renderQueueNear(greeting, n), nil
	}
	return RenderedMail{}, fmt.Errorf("%w: %q", ErrUnknownNotificationKind, kind)
}

func renderAppointment(kind, greeting string, n domain.AppointmentNotice) RenderedMail {
	details := fmt.Sprintf("予約番号: %s\n診察日時: %s\n診療科: %s\n担当医: %s\n", n.AppointmentNo, formatJST(n.StartAt), n.Department, n.Doctor)
	var subject, lead string
	switch kind {
	case domain.KindAppointmentConfirmed:
		subject = "【予約確認】診察予約を受け付けました"
		lead = "診察のご予約を受け付けました。"
	case domain.KindAppointmentCanceled:
		subject = "【キャンセル】診察予約をキャンセルしました"
		lead = "次の診察予約をキャンセルしました。"
	case domain.KindAppointmentRescheduled:
		subject = "【予約変更】診察予約の日時を変更しました"
		lead = "診察予約の日時を変更しました。"
		if n.PreviousStartAt != nil {
			details = fmt.Sprintf("変更前: %s\n変更後: %s\n", formatJST(*n.PreviousStartAt), formatJST(n.StartAt)) +
				fmt.Sprintf("予約番号: %s\n診療科: %s\n担当医: %s\n", n.AppointmentNo, n.Department, n.Doctor)
		}
	case domain.KindAppointmentReminder:
		subject = "【明日の診察】ご予約のお知らせ"
		lead = "明日、次の診察のご予約があります。"
	case domain.KindClinicClosure:
		// 休診の理由(スタッフの自由記述)は意図しない情報を含み得るため、定型文のみとする
		subject = "【休診のお知らせ】ご予約の診察日は休診となりました"
		lead = "ご予約の診察日時が休診となりました。ご不便をおかけし申し訳ありません。"
		body := greeting + "\n\n" + lead + "\n\n" + details + "\n" +
			"お手数ですが、予約画面から別の日時をご予約いただくか、医療機関までご連絡ください。\n"
		return RenderedMail{Subject: subject, Body: body}
	}
	body := greeting + "\n\n" + lead + "\n\n" + details + "\n" +
		"体調や予定が変わった場合は、予約画面から変更・キャンセルできます(期限があります)。\n" +
		"このメールに心当たりがない場合は、お手数ですが医療機関までご連絡ください。\n"
	return RenderedMail{Subject: subject, Body: body}
}

func renderQueueNear(greeting string, n domain.QueueNearNotice) RenderedMail {
	status := fmt.Sprintf("あと%d人でお呼びします。", n.AheadCount)
	if n.AheadCount <= 0 {
		status = "次にお呼びします。"
	}
	body := greeting + "\n\n診察の順番が近づいています。" + status + "\n\n" +
		fmt.Sprintf("診療科: %s\n受付番号: %d\n", n.Department, n.TicketNo) +
		"\n院内でお待ちいただくか、呼び出しに間に合うようにご来院ください。\n"
	return RenderedMail{Subject: "【順番のお知らせ】まもなくお呼びします", Body: body}
}
