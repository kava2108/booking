package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

const (
	maxCancelDeadlineMinutes = 60 * 24 * 31 // 31 日
	maxBookingWindowDays     = 365
)

// ScheduleInput は 1 つの診療時間帯。Weekday は 0=日曜。時刻は "HH:MM"（JST）。
type ScheduleInput struct {
	Weekday     int
	StartTime   string
	EndTime     string
	SlotMinutes int
	Capacity    int
}

// PutSchedulesInput は医師の診療スケジュールの置き換え。
type PutSchedulesInput struct {
	DoctorID  int64
	Schedules []ScheduleInput
}

// DepartmentSettingsInput は診療科ごとの設定。省略した項目は未設定(NULL)に戻る（診療科の設定は全項目の置き換え）。
type DepartmentSettingsInput struct {
	ID                     int64
	StandardConsultMinutes *int
	WalkinCutoff           *string // "HH:MM"
	WalkinCapacity         *int
}

// SettingsInput は受付期間・キャンセル期限・診療科設定。nil の項目は現在値を維持する。
type SettingsInput struct {
	CancelDeadlineMinutes *int
	BookingWindowDays     *int
	Departments           []DepartmentSettingsInput
}

// Settings は保存後の医療機関設定。
type Settings struct {
	CancelDeadlineMinutes int
	BookingWindowDays     int
}

// AdminService はクリニック管理者の設定操作。
type AdminService interface {
	PutSchedules(ctx context.Context, in PutSchedulesInput) error
	PutSettings(ctx context.Context, in SettingsInput) (*Settings, error)
}

// SlotGenerator は保存後に枠を生成するために使う。
type SlotGenerator interface {
	GenerateUpcoming(ctx context.Context) (int, error)
}

type adminService struct {
	repos repository.Repositories
	tx    repository.TxRunner
	gen   SlotGenerator
}

func NewAdminService(repos repository.Repositories, tx repository.TxRunner, gen SlotGenerator, _ ...ClinicOption) AdminService {
	return &adminService{repos: repos, tx: tx, gen: gen}
}

// PutSchedules: 医師の確認 → 入力検証 → 既存スケジュールを置き換え（同一 Tx）→ 枠生成。
// 検証エラーのときは何も変更しない。既に予約が入った枠は削除しない（新規生成のみ）。
func (s *adminService) PutSchedules(ctx context.Context, in PutSchedulesInput) error {
	if _, err := s.repos.Admin.GetDoctor(ctx, in.DoctorID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return validationError("doctor_id", "unknown doctor")
		}
		return err
	}
	rows, verr := validateSchedules(in.Schedules)
	if verr != nil {
		return verr
	}
	err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		if err := r.Admin.DeleteDoctorSchedules(ctx, in.DoctorID); err != nil {
			return err
		}
		for _, row := range rows {
			row.DoctorID = in.DoctorID
			if err := r.Admin.CreateDoctorSchedule(ctx, row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := s.gen.GenerateUpcoming(ctx); err != nil {
		return fmt.Errorf("schedules saved but slot generation failed: %w", err)
	}
	return nil
}

type interval struct {
	idx        int
	start, end time.Duration
}

func validateSchedules(in []ScheduleInput) ([]db.CreateDoctorScheduleParams, *ValidationError) {
	ve := &ValidationError{}
	add := func(i int, field, msg string) {
		ve.Fields = append(ve.Fields, FieldError{Field: fmt.Sprintf("schedules[%d].%s", i, field), Message: msg})
	}
	var rows []db.CreateDoctorScheduleParams
	byDay := map[int][]interval{}

	for i, sc := range in {
		ok := true
		if sc.Weekday < 0 || sc.Weekday > 6 {
			add(i, "weekday", "must be between 0 (Sunday) and 6 (Saturday)")
			ok = false
		}
		start, err1 := domain.ParseClock(sc.StartTime)
		if err1 != nil {
			add(i, "start_time", "must be HH:MM")
			ok = false
		}
		end, err2 := domain.ParseClock(sc.EndTime)
		if err2 != nil {
			add(i, "end_time", "must be HH:MM")
			ok = false
		}
		if sc.SlotMinutes <= 0 {
			add(i, "slot_minutes", "must be positive")
			ok = false
		}
		if sc.Capacity <= 0 {
			add(i, "capacity", "must be positive")
			ok = false
		}
		if err1 == nil && err2 == nil {
			if end <= start {
				add(i, "end_time", "must be after start_time")
				ok = false
			} else if sc.SlotMinutes > 0 && time.Duration(sc.SlotMinutes)*time.Minute > end-start {
				add(i, "slot_minutes", "must not exceed the length of the time range")
				ok = false
			}
		}
		if !ok {
			continue
		}
		byDay[sc.Weekday] = append(byDay[sc.Weekday], interval{i, start, end})
		rows = append(rows, db.CreateDoctorScheduleParams{
			Weekday: int16(sc.Weekday), StartTime: clockValue(start), EndTime: clockValue(end),
			SlotMinutes: int32(sc.SlotMinutes), Capacity: int32(sc.Capacity),
		})
	}

	// 同じ曜日の時間帯が重なっていない（隣接は可）
	for _, list := range byDay {
		sort.Slice(list, func(a, b int) bool { return list[a].start < list[b].start })
		for k := 1; k < len(list); k++ {
			if list[k].start < list[k-1].end {
				later := list[k].idx
				if list[k-1].idx > later {
					later = list[k-1].idx
				}
				ve.Fields = append(ve.Fields, FieldError{Field: fmt.Sprintf("schedules[%d]", later), Message: "overlaps another time range on the same weekday"})
			}
		}
	}
	if len(ve.Fields) > 0 {
		return nil, ve
	}
	return rows, nil
}

func clockValue(d time.Duration) pgtype.Time {
	return pgtype.Time{Microseconds: d.Microseconds(), Valid: true}
}

// PutSettings: 入力検証（何も呼ばずに）→ 同一 Tx で医療機関設定と診療科設定を更新。
func (s *adminService) PutSettings(ctx context.Context, in SettingsInput) (*Settings, error) {
	ve := &ValidationError{}
	if in.CancelDeadlineMinutes != nil && (*in.CancelDeadlineMinutes < 0 || *in.CancelDeadlineMinutes > maxCancelDeadlineMinutes) {
		ve.Fields = append(ve.Fields, FieldError{"cancel_deadline_minutes", fmt.Sprintf("must be between 0 and %d", maxCancelDeadlineMinutes)})
	}
	if in.BookingWindowDays != nil && (*in.BookingWindowDays < 1 || *in.BookingWindowDays > maxBookingWindowDays) {
		ve.Fields = append(ve.Fields, FieldError{"booking_window_days", fmt.Sprintf("must be between 1 and %d", maxBookingWindowDays)})
	}
	deptParams := make([]db.UpdateDepartmentSettingsParams, len(in.Departments))
	for i, d := range in.Departments {
		p := db.UpdateDepartmentSettingsParams{ID: d.ID}
		if d.StandardConsultMinutes != nil {
			if *d.StandardConsultMinutes < 1 {
				ve.Fields = append(ve.Fields, FieldError{fmt.Sprintf("departments[%d].standard_consult_minutes", i), "must be positive"})
			}
			p.StandardConsultMinutes = pgtype.Int4{Int32: int32(*d.StandardConsultMinutes), Valid: true}
		}
		if d.WalkinCutoff != nil {
			c, err := domain.ParseClock(*d.WalkinCutoff)
			if err != nil {
				ve.Fields = append(ve.Fields, FieldError{fmt.Sprintf("departments[%d].walkin_cutoff", i), "must be HH:MM"})
			}
			p.WalkinCutoff = clockValue(c)
		}
		if d.WalkinCapacity != nil {
			if *d.WalkinCapacity < 1 {
				ve.Fields = append(ve.Fields, FieldError{fmt.Sprintf("departments[%d].walkin_capacity", i), "must be positive"})
			}
			p.WalkinCapacity = pgtype.Int4{Int32: int32(*d.WalkinCapacity), Valid: true}
		}
		deptParams[i] = p
	}
	if len(ve.Fields) > 0 {
		return nil, ve
	}

	var out *Settings
	err := s.tx.WithinTx(ctx, func(r repository.Repositories) error {
		cur, err := r.Admin.GetClinicSettings(ctx)
		if err != nil {
			return err
		}
		p := db.UpdateClinicSettingsParams{CancelDeadlineMinutes: cur.CancelDeadlineMinutes, BookingWindowDays: cur.BookingWindowDays}
		if in.CancelDeadlineMinutes != nil {
			p.CancelDeadlineMinutes = int32(*in.CancelDeadlineMinutes)
		}
		if in.BookingWindowDays != nil {
			p.BookingWindowDays = int32(*in.BookingWindowDays)
		}
		updated, err := r.Admin.UpdateClinicSettings(ctx, p)
		if err != nil {
			return err
		}
		for i, dp := range deptParams {
			if _, err := r.Admin.UpdateDepartmentSettings(ctx, dp); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return validationError(fmt.Sprintf("departments[%d].id", i), "unknown department")
				}
				return err
			}
		}
		out = &Settings{CancelDeadlineMinutes: int(updated.CancelDeadlineMinutes), BookingWindowDays: int(updated.BookingWindowDays)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
