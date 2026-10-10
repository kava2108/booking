package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

var (
	// ErrSlotFull は枠が満席になったこと（HTTP 409 slot_full）。
	ErrSlotFull = errors.New("slot is full")
	// ErrDuplicateAppointment は同じ時刻に有効な予約が既にあること（HTTP 409 duplicate_appointment）。
	ErrDuplicateAppointment = errors.New("duplicate appointment")
	// ErrCancelDeadlinePassed はキャンセル/変更の期限を過ぎていること（HTTP 422 cancel_deadline_passed）。
	ErrCancelDeadlinePassed = errors.New("cancel deadline passed")
	// ErrAppointmentNotFound は予約が存在しない、他人の予約、または有効でないこと（HTTP 404）。
	ErrAppointmentNotFound = errors.New("appointment not found")

	errAppointmentNoCollision = errors.New("appointment number collision")
)

// maxNumberAttempts は予約番号の衝突時にトランザクションごと再試行する最大回数。
const maxNumberAttempts = 5

// CreateAppointmentInput は予約確定の入力。
type CreateAppointmentInput struct {
	SlotID      uuid.UUID
	VisitType   string // first | revisit
	SymptomNote string // 要配慮個人情報。通知には含めない
}

// AppointmentResult は確定・変更後の予約。
type AppointmentResult struct {
	ID            uuid.UUID
	AppointmentNo string
	SlotStart     time.Time
	Department    string
	Doctor        string
}

// AppointmentSummary は本人向けの予約一覧の 1 件（症状メモを含む。他人には返さない）。
type AppointmentSummary struct {
	ID            uuid.UUID
	AppointmentNo string
	SlotStart     time.Time
	Department    string
	Doctor        string
	VisitType     string
	Status        string
	SymptomNote   string
	Cancelable    bool
}

// AppointmentService は患者の予約操作。枠の確保と予約の作成を 1 トランザクションで行い、
// 通知はアウトボックスに同一 Tx で登録する（送信は別ワーカー）。
// IMP 12-clinic-reservation-appointment-api TASK-0001
type AppointmentService interface {
	Create(ctx context.Context, patientID uuid.UUID, in CreateAppointmentInput) (*AppointmentResult, error)
	ListMine(ctx context.Context, patientID uuid.UUID) ([]AppointmentSummary, error)
	Cancel(ctx context.Context, patientID, appointmentID uuid.UUID) error
	Reschedule(ctx context.Context, patientID, appointmentID, newSlotID uuid.UUID) (*AppointmentResult, error)
}

type appointmentService struct {
	repos   repository.Repositories
	tx      repository.TxRunner
	now     func() time.Time
	numbers func(slotStart time.Time) string
}

// WithAppointmentNumbers はテスト用に予約番号の生成を差し替える。
func WithAppointmentNumbers(f func(slotStart time.Time) string) ClinicOption {
	return func(o *clinicOptions) { o.numbers = f }
}

func NewAppointmentService(repos repository.Repositories, tx repository.TxRunner, opts ...ClinicOption) AppointmentService {
	o := newClinicOptions(opts)
	numbers := o.numbers
	if numbers == nil {
		numbers = randomAppointmentNo
	}
	return &appointmentService{repos: repos, tx: tx, now: o.now, numbers: numbers}
}

// randomAppointmentNo は A-<診療日(JST)>-<4 桁乱数>。衝突は一意制約で検出して再試行する。
func randomAppointmentNo(slotStart time.Time) string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		n = big.NewInt(time.Now().UnixNano() % 10000)
	}
	return fmt.Sprintf("A-%s-%04d", slotStart.In(domain.JST).Format("20060102"), n.Int64())
}

// bookable は枠が予約対象か（休診でない・未来・受付期間内）を検証する。満席の判定は ReserveSlot に任せる。
func bookable(d db.GetSlotDetailRow, now time.Time, windowDays int32) error {
	start := d.StartAt.Time
	switch {
	case d.Status != "open":
		return validationError("slot_id", "slot is not available: closed")
	case !start.After(now):
		return validationError("slot_id", "slot is not available: past_cutoff")
	case domain.StartOfDay(start).After(domain.StartOfDay(now).AddDate(0, 0, int(windowDays))):
		return validationError("slot_id", "slot is not available: outside_window")
	}
	return nil
}

func (s *appointmentService) notice(a db.Appointment, d db.GetSlotDetailRow) domain.AppointmentNotice {
	return domain.AppointmentNotice{
		AppointmentID: a.ID.String(), AppointmentNo: a.AppointmentNo,
		StartAt: d.StartAt.Time.In(domain.JST), Department: d.DepartmentName, Doctor: d.DoctorName,
	}
}

func enqueue(ctx context.Context, r repository.Repositories, userID uuid.UUID, kind string, n domain.AppointmentNotice) error {
	payload, err := json.Marshal(n)
	if err != nil {
		return err
	}
	_, err = r.Notifications.EnqueueNotification(ctx, db.EnqueueNotificationParams{UserID: userID, Kind: kind, Payload: payload})
	return err
}

func result(a db.Appointment, d db.GetSlotDetailRow) *AppointmentResult {
	return &AppointmentResult{ID: a.ID, AppointmentNo: a.AppointmentNo, SlotStart: d.StartAt.Time.In(domain.JST), Department: d.DepartmentName, Doctor: d.DoctorName}
}

// Create: 検証 → 同時刻の重複 → 枠を条件付き UPDATE で確保 → 予約作成 → 通知登録（すべて 1 Tx）。
// 失敗時はすべて巻き戻る（枠だけ確保された状態を作らない）。予約番号の衝突のみ Tx ごと再試行する。
func (s *appointmentService) Create(ctx context.Context, patientID uuid.UUID, in CreateAppointmentInput) (*AppointmentResult, error) {
	for attempt := 0; attempt < maxNumberAttempts; attempt++ {
		res, err := s.createOnce(ctx, patientID, in)
		if errors.Is(err, errAppointmentNoCollision) {
			continue
		}
		return res, err
	}
	return nil, errors.New("could not allocate a unique appointment number")
}

func (s *appointmentService) createOnce(ctx context.Context, patientID uuid.UUID, in CreateAppointmentInput) (*AppointmentResult, error) {
	var out *AppointmentResult
	err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		cfg, err := r.Admin.GetClinicSettings(ctx)
		if err != nil {
			return err
		}
		d, err := r.Slots.GetSlotDetail(ctx, in.SlotID)
		if errors.Is(err, pgx.ErrNoRows) {
			return validationError("slot_id", "unknown slot")
		}
		if err != nil {
			return err
		}
		if err := bookable(d, s.now(), cfg.BookingWindowDays); err != nil {
			return err
		}
		n, err := r.Appointments.CountPatientBookingsAt(ctx, db.CountPatientBookingsAtParams{PatientID: patientID, StartAt: d.StartAt})
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrDuplicateAppointment
		}
		if _, err := r.Slots.ReserveSlot(ctx, in.SlotID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSlotFull
			}
			return err
		}
		a, err := r.Appointments.CreateAppointment(ctx, db.CreateAppointmentParams{
			AppointmentNo: s.numbers(d.StartAt.Time), PatientID: patientID, SlotID: in.SlotID, VisitType: in.VisitType, SymptomNote: in.SymptomNote,
		})
		if err != nil {
			if c, ok := repository.UniqueViolationConstraint(err); ok {
				switch c {
				case repository.ConstraintAppointmentNo:
					return errAppointmentNoCollision
				case repository.ConstraintActivePatientSlot:
					return ErrDuplicateAppointment
				}
			}
			return err
		}
		if err := enqueue(ctx, r, patientID, domain.KindAppointmentConfirmed, s.notice(a, d)); err != nil {
			return err
		}
		out = result(a, d)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *appointmentService) ListMine(ctx context.Context, patientID uuid.UUID) ([]AppointmentSummary, error) {
	cfg, err := s.repos.Admin.GetClinicSettings(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.repos.Appointments.ListAppointmentsByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	deadline := time.Duration(cfg.CancelDeadlineMinutes) * time.Minute
	out := make([]AppointmentSummary, 0, len(rows))
	for _, r := range rows {
		start := r.StartAt.Time
		out = append(out, AppointmentSummary{
			ID: r.ID, AppointmentNo: r.AppointmentNo, SlotStart: start.In(domain.JST), Department: r.DepartmentName, Doctor: r.DoctorName,
			VisitType: r.VisitType, Status: r.Status, SymptomNote: r.SymptomNote,
			Cancelable: r.Status == "booked" && now.Before(start.Add(-deadline)),
		})
	}
	return out, nil
}

// beforeDeadline は期限前(開始の cancel_deadline_minutes 前より前)かを返す。期限ちょうどは期限後として扱う。
func beforeDeadline(now, start time.Time, cfg db.ClinicSetting) bool {
	return now.Before(start.Add(-time.Duration(cfg.CancelDeadlineMinutes) * time.Minute))
}

// Cancel: 期限内なら取消 → 枠を解放 → 通知（同一 Tx）。他人の予約・有効でない予約は ErrAppointmentNotFound。
func (s *appointmentService) Cancel(ctx context.Context, patientID, appointmentID uuid.UUID) error {
	return s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		a, err := r.Appointments.GetAppointment(ctx, db.GetAppointmentParams{ID: appointmentID, PatientID: patientID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAppointmentNotFound
		}
		if err != nil {
			return err
		}
		if a.Status != "booked" {
			return ErrAppointmentNotFound
		}
		d, err := r.Slots.GetSlotDetail(ctx, a.SlotID)
		if err != nil {
			return err
		}
		cfg, err := r.Admin.GetClinicSettings(ctx)
		if err != nil {
			return err
		}
		if !beforeDeadline(s.now(), d.StartAt.Time, cfg) {
			return ErrCancelDeadlinePassed
		}
		if _, err := r.Appointments.CancelAppointment(ctx, db.CancelAppointmentParams{ID: appointmentID, PatientID: patientID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAppointmentNotFound // 競合で先に取り消された（二重に解放しない）
			}
			return err
		}
		if err := r.Slots.ReleaseSlot(ctx, a.SlotID); err != nil {
			return err
		}
		return enqueue(ctx, r, patientID, domain.KindAppointmentCanceled, s.notice(a, d))
	})
}

// Reschedule: 新しい枠を確保 → 予約を付け替え → 旧枠を解放 → 通知（同一 Tx）。
// 失敗した場合は旧予約がそのまま残る。取消と同じ期限（元の予約の開始基準）を適用する。
func (s *appointmentService) Reschedule(ctx context.Context, patientID, appointmentID, newSlotID uuid.UUID) (*AppointmentResult, error) {
	var out *AppointmentResult
	err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		a, err := r.Appointments.GetAppointment(ctx, db.GetAppointmentParams{ID: appointmentID, PatientID: patientID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAppointmentNotFound
		}
		if err != nil {
			return err
		}
		if a.Status != "booked" {
			return ErrAppointmentNotFound
		}
		if a.SlotID == newSlotID {
			return validationError("slot_id", "same as the current slot")
		}
		cfg, err := r.Admin.GetClinicSettings(ctx)
		if err != nil {
			return err
		}
		oldD, err := r.Slots.GetSlotDetail(ctx, a.SlotID)
		if err != nil {
			return err
		}
		if !beforeDeadline(s.now(), oldD.StartAt.Time, cfg) {
			return ErrCancelDeadlinePassed
		}
		newD, err := r.Slots.GetSlotDetail(ctx, newSlotID)
		if errors.Is(err, pgx.ErrNoRows) {
			return validationError("slot_id", "unknown slot")
		}
		if err != nil {
			return err
		}
		if err := bookable(newD, s.now(), cfg.BookingWindowDays); err != nil {
			return err
		}
		n, err := r.Appointments.CountPatientBookingsAt(ctx, db.CountPatientBookingsAtParams{PatientID: patientID, StartAt: newD.StartAt})
		if err != nil {
			return err
		}
		allowed := int64(0)
		if oldD.StartAt.Time.Equal(newD.StartAt.Time) {
			allowed = 1 // 同じ時刻の別枠へ移る場合、数えられるのは自分自身の予約
		}
		if n > allowed {
			return ErrDuplicateAppointment
		}
		if _, err := r.Slots.ReserveSlot(ctx, newSlotID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSlotFull
			}
			return err
		}
		moved, err := r.Appointments.RescheduleAppointment(ctx, db.RescheduleAppointmentParams{ID: appointmentID, PatientID: patientID, SlotID: newSlotID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAppointmentNotFound
			}
			if c, ok := repository.UniqueViolationConstraint(err); ok && c == repository.ConstraintActivePatientSlot {
				return ErrDuplicateAppointment
			}
			return err
		}
		if err := r.Slots.ReleaseSlot(ctx, a.SlotID); err != nil {
			return err
		}
		prev := oldD.StartAt.Time.In(domain.JST)
		notice := s.notice(moved, newD)
		notice.PreviousStartAt = &prev
		if err := enqueue(ctx, r, patientID, domain.KindAppointmentRescheduled, notice); err != nil {
			return err
		}
		out = result(moved, newD)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
