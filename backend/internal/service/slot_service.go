package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

// 枠が選べない理由（OpenAPI の Slot.unavailable_reason と一致させる）。
const (
	ReasonFull          = "full"
	ReasonClosed        = "closed"
	ReasonPastCutoff    = "past_cutoff"
	ReasonOutsideWindow = "outside_window"
)

// maxGenerateDays は 1 回の生成で許す最大日数（誤操作で大量生成しない）。
const maxGenerateDays = 366

// SlotQuery は空き枠検索の条件。Date は JST の診療日。
type SlotQuery struct {
	DepartmentID int64
	DoctorID     *int64
	Date         time.Time
}

// SlotView は患者に見せる枠。capacity / booked_count などの内部情報は含めない。
type SlotView struct {
	ID        uuid.UUID
	DoctorID  int64
	StartAt   time.Time
	Available bool
	Reason    string // Available=false のときの理由
}

// SlotService は診療科・医師・枠の参照と、スケジュールからの枠生成を担う。
// IMP 11-clinic-reservation-slot-api-admin-settings TASK-0001
type SlotService interface {
	ListDepartments(ctx context.Context) ([]db.Department, error)
	ListDoctors(ctx context.Context, departmentID int64) ([]db.Doctor, error)
	ListSlots(ctx context.Context, q SlotQuery) ([]SlotView, error)
	// GenerateSlots は from〜to（JST の日付、両端を含む）の枠をスケジュールから冪等に生成し、休診期間を反映する。
	GenerateSlots(ctx context.Context, from, to time.Time) (int, error)
	// GenerateUpcoming は今日から受付期間の最終日までを生成する（定期ジョブ・管理者操作後）。
	GenerateUpcoming(ctx context.Context) (int, error)
}

type slotService struct {
	repos repository.Repositories
	now   func() time.Time
}

func NewSlotService(repos repository.Repositories, _ repository.TxRunner, opts ...ClinicOption) SlotService {
	return &slotService{repos: repos, now: newClinicOptions(opts).now}
}

func (s *slotService) ListDepartments(ctx context.Context) ([]db.Department, error) {
	return s.repos.Admin.ListDepartments(ctx)
}

func (s *slotService) ListDoctors(ctx context.Context, departmentID int64) ([]db.Doctor, error) {
	return s.repos.Admin.ListDoctors(ctx, departmentID)
}

// ListSlots は満席・休診・締切後・受付期間外の枠も理由つきで返す（選択不可として表示するため、REQ-001-AC-3）。
func (s *slotService) ListSlots(ctx context.Context, q SlotQuery) ([]SlotView, error) {
	if _, err := s.repos.Admin.GetDepartment(ctx, q.DepartmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, validationError("department_id", "unknown department")
		}
		return nil, err
	}
	cfg, err := s.repos.Admin.GetClinicSettings(ctx)
	if err != nil {
		return nil, err
	}
	from := domain.StartOfDay(q.Date)
	params := db.ListSlotsParams{DepartmentID: q.DepartmentID, FromAt: tsOf(from), ToAt: tsOf(from.AddDate(0, 0, 1))}
	if q.DoctorID != nil {
		params.DoctorID = pgtype.Int8{Int64: *q.DoctorID, Valid: true}
	}
	rows, err := s.repos.Slots.ListSlots(ctx, params)
	if err != nil {
		return nil, err
	}

	now := s.now()
	lastDay := domain.StartOfDay(now).AddDate(0, 0, int(cfg.BookingWindowDays))
	out := make([]SlotView, 0, len(rows))
	for _, r := range rows {
		v := SlotView{ID: r.ID, DoctorID: r.DoctorID, StartAt: r.StartAt.Time.In(domain.JST)}
		switch {
		case r.Status != "open":
			v.Reason = ReasonClosed
		case !r.StartAt.Time.After(now):
			v.Reason = ReasonPastCutoff
		case domain.StartOfDay(r.StartAt.Time).After(lastDay):
			v.Reason = ReasonOutsideWindow
		case r.BookedCount >= r.Capacity:
			v.Reason = ReasonFull
		default:
			v.Available = true
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *slotService) GenerateUpcoming(ctx context.Context) (int, error) {
	cfg, err := s.repos.Admin.GetClinicSettings(ctx)
	if err != nil {
		return 0, err
	}
	today := domain.StartOfDay(s.now())
	return s.GenerateSlots(ctx, today, today.AddDate(0, 0, int(cfg.BookingWindowDays)))
}

func (s *slotService) GenerateSlots(ctx context.Context, from, to time.Time) (int, error) {
	from, to = domain.StartOfDay(from), domain.StartOfDay(to)
	if to.Before(from) {
		return 0, errors.New("generate slots: to is before from")
	}
	if to.Sub(from) > maxGenerateDays*24*time.Hour {
		return 0, fmt.Errorf("generate slots: range exceeds %d days", maxGenerateDays)
	}

	count := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		schedules, err := s.repos.Admin.ListSchedulesByWeekday(ctx, int16(d.Weekday()))
		if err != nil {
			return count, err
		}
		for _, sc := range schedules {
			slotLen := time.Duration(sc.SlotMinutes) * time.Minute
			start := time.Duration(sc.StartTime.Microseconds) * time.Microsecond
			end := time.Duration(sc.EndTime.Microseconds) * time.Microsecond
			if slotLen <= 0 {
				continue
			}
			// 枠が終了時刻に収まるものだけ作る（終了ちょうどに始まる枠は作らない）
			for off := start; off+slotLen <= end; off += slotLen {
				err := s.repos.Slots.CreateSlotIfAbsent(ctx, db.CreateSlotIfAbsentParams{
					DepartmentID: sc.DepartmentID, DoctorID: sc.DoctorID, StartAt: tsOf(d.Add(off)), Capacity: sc.Capacity,
				})
				if err != nil {
					return count, err
				}
				count++
			}
		}
	}

	// 既存の休診期間を、新しく作った枠にも反映する
	closures, err := s.repos.Admin.ListClosuresFrom(ctx, tsOf(from))
	if err != nil {
		return count, err
	}
	for _, c := range closures {
		if _, err := s.repos.Slots.CloseSlots(ctx, db.CloseSlotsParams{DoctorID: c.DoctorID, FromAt: c.FromAt, ToAt: c.ToAt}); err != nil {
			return count, err
		}
	}
	return count, nil
}

func tsOf(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
