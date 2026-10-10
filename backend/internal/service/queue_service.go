package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

var (
	// ErrQueueEntryNotFound は待機列の項目が存在しないこと（HTTP 404）。
	ErrQueueEntryNotFound = errors.New("queue entry not found")
	// ErrInvalidTransition は現在の状態からその操作ができないこと（HTTP 409）。
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrAlreadyInQueue は同じ予約・同じ患者が既に列に並んでいること（HTTP 409）。
	ErrAlreadyInQueue = errors.New("already in queue")
	// ErrWalkInClosed は当日受付を締め切った/定員に達したこと（HTTP 422）。
	ErrWalkInClosed = errors.New("walk-in registration is closed")

	errTicketCollision = errors.New("ticket number collision")
)

// InvalidTransitionError は不正な遷移と、そのときの現在の状態を運ぶ。
type InvalidTransitionError struct{ Current string }

func (e *InvalidTransitionError) Error() string        { return "invalid transition from " + e.Current }
func (e *InvalidTransitionError) Is(target error) bool { return target == ErrInvalidTransition }

// WalkInClosedError は当日受付ができない理由（after_cutoff | capacity_reached）。
type WalkInClosedError struct{ Reason string }

func (e *WalkInClosedError) Error() string        { return "walk-in closed: " + e.Reason }
func (e *WalkInClosedError) Is(target error) bool { return target == ErrWalkInClosed }

const (
	maxTicketAttempts  = 5
	maxDisplayNameRune = 100
)

// QueueState は操作後の待機列項目の状態。
type QueueState struct {
	ID     uuid.UUID
	Status string
}

// WalkInInput は当日受付の入力。患者本人は PatientID、スタッフ代行は PatientID か DisplayName（無アカウント）。
type WalkInInput struct {
	DepartmentID int64
	PatientID    *uuid.UUID
	DisplayName  string
}

// WalkInTicket は当日受付の結果。
type WalkInTicket struct {
	QueueEntryID uuid.UUID
	TicketNo     int
	AheadCount   int
}

// StaffQueueEntry はスタッフ向けの待機列の 1 件（氏名・症状メモを含む。公開には使わない）。
type StaffQueueEntry struct {
	ID          uuid.UUID
	TicketNo    int
	Source      string
	Status      string
	PatientName string
	SymptomNote string
	CheckedInAt time.Time
}

// QueueService は来院受付・呼出・診察・当日受付と、スタッフ向け待機列。
// 予約患者と当日受付を 1 つの列(queue_entries)で扱い、順序は sort_time で決める:
//   - 予約: max(予約時刻, 来院受付時刻)  → 早着でも予約時刻、遅刻者は先着の当日患者を追い越さない
//   - 当日受付: 受付時刻
//
// 状態遷移のルールは SQL(WHERE status = 現在値)に集約している。
// IMP 13-clinic-reservation-queue-staff-walkin TASK-0001
type QueueService interface {
	CheckIn(ctx context.Context, appointmentID uuid.UUID) (*QueueState, error)
	Call(ctx context.Context, entryID uuid.UUID) (*QueueState, error)
	Start(ctx context.Context, entryID uuid.UUID) (*QueueState, error)
	Complete(ctx context.Context, entryID uuid.UUID) (*QueueState, error)
	CreateWalkIn(ctx context.Context, in WalkInInput) (*WalkInTicket, error)
	ListStaffQueue(ctx context.Context, departmentID int64) ([]StaffQueueEntry, error)
}

type queueService struct {
	repos repository.Repositories
	tx    repository.TxRunner
	now   func() time.Time
}

func NewQueueService(repos repository.Repositories, tx repository.TxRunner, opts ...ClinicOption) QueueService {
	return &queueService{repos: repos, tx: tx, now: newClinicOptions(opts).now}
}

func serviceDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: domain.StartOfDay(t), Valid: true}
}

// retryTickets は受付番号の衝突時に、トランザクションごと再試行する（Tx 内の再試行は中断済み Tx では不可）。
// 診療科の行ロックで通常は衝突しないが、最終防壁の一意制約に当たった場合の保険。
func retryTickets[T any](f func() (T, error)) (T, error) {
	var zero T
	for i := 0; i < maxTicketAttempts; i++ {
		v, err := f()
		if errors.Is(err, errTicketCollision) {
			continue
		}
		return v, err
	}
	return zero, errors.New("could not allocate a unique ticket number")
}

// enqueue は診療科をロックして採番し、待機列に追加する（トランザクション内で呼ぶ）。
func (s *queueService) enqueue(ctx context.Context, r repository.Repositories, p db.EnqueueEntryParams) (db.QueueEntry, error) {
	no, err := r.Queue.NextTicketNo(ctx, db.NextTicketNoParams{DepartmentID: p.DepartmentID, ServiceDate: p.ServiceDate})
	if err != nil {
		return db.QueueEntry{}, err
	}
	p.TicketNo = no
	q, err := r.Queue.EnqueueEntry(ctx, p)
	if err != nil {
		if c, ok := repository.UniqueViolationConstraint(err); ok {
			switch c {
			case repository.ConstraintQueueTicket:
				return db.QueueEntry{}, errTicketCollision
			case repository.ConstraintQueueAppointment:
				return db.QueueEntry{}, ErrAlreadyInQueue
			}
		}
		return db.QueueEntry{}, err
	}
	return q, nil
}

// CheckIn: 予約患者の来院受付。当日の有効な予約のみ。
func (s *queueService) CheckIn(ctx context.Context, appointmentID uuid.UUID) (*QueueState, error) {
	return retryTickets(func() (*QueueState, error) {
		var out *QueueState
		err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
			a, err := r.Appointments.GetAppointmentWithSlot(ctx, appointmentID)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAppointmentNotFound
			}
			if err != nil {
				return err
			}
			if a.Status != "booked" {
				return validationError("id", "appointment is not active: "+a.Status)
			}
			now := s.now()
			if !domain.StartOfDay(a.StartAt.Time).Equal(domain.StartOfDay(now)) {
				return validationError("id", "appointment is not for today")
			}
			if _, err := r.Admin.LockDepartment(ctx, a.DepartmentID); err != nil {
				return err
			}
			sortTime := a.StartAt.Time
			if now.After(sortTime) {
				sortTime = now // 遅刻者は受付時刻で並ぶ
			}
			q, err := s.enqueue(ctx, r, db.EnqueueEntryParams{
				DepartmentID: a.DepartmentID, AppointmentID: pgtype.UUID{Bytes: a.ID, Valid: true}, PatientID: pgtype.UUID{Bytes: a.PatientID, Valid: true},
				Source: "reservation", ServiceDate: serviceDate(now), SortTime: tsOf(sortTime), CheckedInAt: tsOf(now),
			})
			if err != nil {
				return err
			}
			out = &QueueState{ID: q.ID, Status: q.Status}
			return nil
		})
		return out, err
	})
}

func (s *queueService) Call(ctx context.Context, id uuid.UUID) (*QueueState, error) {
	return s.transition(ctx, id, func(r repository.Repositories) (db.QueueEntry, error) { return r.Queue.CallQueueEntry(ctx, id) })
}

func (s *queueService) Start(ctx context.Context, id uuid.UUID) (*QueueState, error) {
	return s.transition(ctx, id, func(r repository.Repositories) (db.QueueEntry, error) { return r.Queue.StartQueueEntry(ctx, id) })
}

// Complete: 診察完了。予約由来なら予約も completed にする（同一 Tx）。
func (s *queueService) Complete(ctx context.Context, id uuid.UUID) (*QueueState, error) {
	return s.transition(ctx, id, func(r repository.Repositories) (db.QueueEntry, error) {
		q, err := r.Queue.CompleteQueueEntry(ctx, id)
		if err != nil {
			return q, err
		}
		if q.AppointmentID.Valid {
			if _, err := r.Appointments.CompleteAppointment(ctx, uuid.UUID(q.AppointmentID.Bytes)); err != nil {
				return db.QueueEntry{}, err
			}
		}
		return q, nil
	})
}

// transition は条件付き UPDATE の結果を解釈する。0 行なら、項目が無ければ NotFound、あれば不正遷移。
func (s *queueService) transition(ctx context.Context, id uuid.UUID, do func(repository.Repositories) (db.QueueEntry, error)) (*QueueState, error) {
	var out *QueueState
	err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		q, err := do(r)
		if errors.Is(err, pgx.ErrNoRows) {
			cur, gerr := r.Queue.GetQueueEntry(ctx, id)
			if errors.Is(gerr, pgx.ErrNoRows) {
				return ErrQueueEntryNotFound
			}
			if gerr != nil {
				return gerr
			}
			return &InvalidTransitionError{Current: cur.Status}
		}
		if err != nil {
			return err
		}
		out = &QueueState{ID: q.ID, Status: q.Status}
		return nil
	})
	return out, err
}

// CreateWalkIn: 当日受付。締切時刻・定員・重複を検証し、診療科の行ロックで採番と定員判定を直列化する。
func (s *queueService) CreateWalkIn(ctx context.Context, in WalkInInput) (*WalkInTicket, error) {
	name := strings.TrimSpace(in.DisplayName)
	switch {
	case in.PatientID == nil && name == "":
		return nil, validationError("display_name", "patient_id or display_name is required")
	case utf8.RuneCountInString(name) > maxDisplayNameRune:
		return nil, validationError("display_name", fmt.Sprintf("must be at most %d characters", maxDisplayNameRune))
	}
	dept, err := s.repos.Admin.GetDepartment(ctx, in.DepartmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, validationError("department_id", "unknown department")
	}
	if err != nil {
		return nil, err
	}

	return retryTickets(func() (*WalkInTicket, error) {
		var out *WalkInTicket
		err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
			if _, err := r.Admin.LockDepartment(ctx, in.DepartmentID); err != nil {
				return err
			}
			now := s.now()
			date := serviceDate(now)
			if dept.WalkinCutoff.Valid {
				cutoff := time.Duration(dept.WalkinCutoff.Microseconds) * time.Microsecond
				if now.Sub(domain.StartOfDay(now)) >= cutoff {
					return &WalkInClosedError{Reason: "after_cutoff"}
				}
			}
			if dept.WalkinCapacity.Valid {
				n, err := r.Queue.CountWalkInsForDate(ctx, db.CountWalkInsForDateParams{DepartmentID: in.DepartmentID, ServiceDate: date})
				if err != nil {
					return err
				}
				if n >= int64(dept.WalkinCapacity.Int32) {
					return &WalkInClosedError{Reason: "capacity_reached"}
				}
			}
			p := db.EnqueueEntryParams{
				DepartmentID: in.DepartmentID, Source: "walk_in", ServiceDate: date, SortTime: tsOf(now), CheckedInAt: tsOf(now),
			}
			if in.PatientID != nil {
				p.PatientID = pgtype.UUID{Bytes: *in.PatientID, Valid: true}
				n, err := r.Queue.CountActiveQueueEntriesForPatient(ctx, db.CountActiveQueueEntriesForPatientParams{DepartmentID: in.DepartmentID, ServiceDate: date, PatientID: p.PatientID})
				if err != nil {
					return err
				}
				if n > 0 {
					return ErrAlreadyInQueue
				}
			}
			if name != "" {
				p.DisplayName = pgtype.Text{String: name, Valid: true}
			}
			q, err := s.enqueue(ctx, r, p)
			if err != nil {
				return err
			}
			ahead, err := r.Queue.CountAhead(ctx, db.CountAheadParams{DepartmentID: in.DepartmentID, ServiceDate: date, SortTime: q.SortTime})
			if err != nil {
				return err
			}
			out = &WalkInTicket{QueueEntryID: q.ID, TicketNo: int(q.TicketNo), AheadCount: int(ahead)}
			return nil
		})
		return out, err
	})
}

// ListStaffQueue: 今日の待機列（待機中・呼出済み・診察中）。氏名・症状メモを含むため StaffOnly + 監査ログで保護する。
func (s *queueService) ListStaffQueue(ctx context.Context, departmentID int64) ([]StaffQueueEntry, error) {
	rows, err := s.repos.Queue.ListStaffQueue(ctx, db.ListStaffQueueParams{DepartmentID: departmentID, ServiceDate: serviceDate(s.now())})
	if err != nil {
		return nil, err
	}
	out := make([]StaffQueueEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, StaffQueueEntry{
			ID: r.ID, TicketNo: int(r.TicketNo), Source: r.Source, Status: r.Status,
			PatientName: r.PatientName, SymptomNote: r.SymptomNote, CheckedInAt: r.CheckedInAt.Time.In(domain.JST),
		})
	}
	return out, nil
}
