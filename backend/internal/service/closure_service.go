package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

const (
	maxClosureDays   = 366
	maxClosureReason = 200
)

// ClosureInput は休診・医師不在の設定。DoctorID が nil なら医療機関全体の休診。期間は [From, To)。
type ClosureInput struct {
	DoctorID *int64
	From, To time.Time
	Reason   string // スタッフ用の記録。患者への通知には含めない
}

// ClosureResult は設定の結果。Notified は今回新たに休診になった枠の予約者への通知数。
type ClosureResult struct{ ClosedSlots, Notified int }

// ClosureService は休診の設定。枠を閉じ、影響する予約者へ通知を登録し、監査ログに残す（すべて同一 Tx）。
// 予約は自動キャンセルしない（患者の意思確認が必要なため、通知で再予約を案内する）。
// IMP 16-clinic-reservation-closures TASK-0001 / REQ-005-AC-3
type ClosureService interface {
	Create(ctx context.Context, actorID uuid.UUID, in ClosureInput) (*ClosureResult, error)
}

type closureService struct {
	repos repository.Repositories
	tx    repository.TxRunner
	now   func() time.Time
}

func NewClosureService(repos repository.Repositories, tx repository.TxRunner, opts ...ClinicOption) ClosureService {
	return &closureService{repos: repos, tx: tx, now: newClinicOptions(opts).now}
}

func (s *closureService) Create(ctx context.Context, actorID uuid.UUID, in ClosureInput) (*ClosureResult, error) {
	switch {
	case !in.To.After(in.From):
		return nil, validationError("to_at", "must be after from_at")
	case !in.To.After(s.now()):
		return nil, validationError("to_at", "must be in the future")
	case in.To.Sub(in.From) > maxClosureDays*24*time.Hour:
		return nil, validationError("to_at", fmt.Sprintf("the period must not exceed %d days", maxClosureDays))
	case utf8.RuneCountInString(in.Reason) > maxClosureReason:
		return nil, validationError("reason", fmt.Sprintf("must be at most %d characters", maxClosureReason))
	}
	var doctor pgtype.Int8
	target := "clinic"
	if in.DoctorID != nil {
		if _, err := s.repos.Admin.GetDoctor(ctx, *in.DoctorID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, validationError("doctor_id", "unknown doctor")
			}
			return nil, err
		}
		doctor = pgtype.Int8{Int64: *in.DoctorID, Valid: true}
		target = fmt.Sprintf("doctor:%d", *in.DoctorID)
	}

	var out *ClosureResult
	err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		if err := r.Admin.CreateClosure(ctx, db.CreateClosureParams{DoctorID: doctor, FromAt: tsOf(in.From), ToAt: tsOf(in.To), Reason: in.Reason}); err != nil {
			return err
		}
		closed, err := r.Slots.CloseSlots(ctx, db.CloseSlotsParams{DoctorID: doctor, FromAt: tsOf(in.From), ToAt: tsOf(in.To)})
		if err != nil {
			return err
		}
		newlyClosed := make(map[uuid.UUID]bool, len(closed))
		for _, id := range closed {
			newlyClosed[id] = true
		}
		booked, err := r.Appointments.ListBookedAppointmentsForNotice(ctx, db.ListBookedAppointmentsForNoticeParams{DoctorID: doctor, FromAt: tsOf(in.From), ToAt: tsOf(in.To)})
		if err != nil {
			return err
		}
		notified := 0
		for _, b := range booked {
			if !newlyClosed[b.SlotID] {
				continue // 既に休診済みの枠の予約者には、その時点で通知済み（二重に通知しない）
			}
			payload, err := json.Marshal(domain.AppointmentNotice{
				AppointmentID: b.ID.String(), AppointmentNo: b.AppointmentNo, StartAt: b.StartAt.Time.In(domain.JST),
				Department: b.DepartmentName, Doctor: b.DoctorName,
			})
			if err != nil {
				return err
			}
			if _, err := r.Notifications.EnqueueNotification(ctx, db.EnqueueNotificationParams{UserID: b.PatientID, Kind: domain.KindClinicClosure, Payload: payload}); err != nil {
				return err
			}
			notified++
		}
		if err := r.Admin.InsertAuditLog(ctx, db.InsertAuditLogParams{ActorID: pgUUID(actorID), Action: "create_closure", Target: target}); err != nil {
			return err
		}
		out = &ClosureResult{ClosedSlots: len(closed), Notified: notified}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
