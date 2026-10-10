package service

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

const (
	waitCacheTTL       = 10 * time.Second // 公開応答のサーバー側キャッシュ（フロントは 30 秒間隔で更新）
	waitSampleLimit    = 20               // 平均に使う直近の診察件数
	waitMinSamples     = 3                // これ未満は実績を使わず標準診察時間にフォールバック
	waitSampleMaxSecs  = 3 * 3600         // 1 件の所要時間の上限（完了の押し忘れで平均を歪めない）
	WaitBasisActual    = "actual"
	WaitBasisStandard  = "standard"
	WaitStatusAvail    = "available"
	WaitStatusUnavail  = "unavailable"
	WaitReasonClosed   = "closed"
	WaitReasonBefore   = "before_hours"
	WaitReasonNoData   = "no_data"
	QueueNotCheckedIn  = "not_checked_in"
	queueStatusWaiting = "waiting"
	queueStatusCalled  = "called"
)

// DepartmentWait は診療科ごとの現在の待ち時間。Status=unavailable のときは WaitMinutes / Basis は無意味。
type DepartmentWait struct {
	DepartmentID int64
	Name         string
	Status       string // available | unavailable
	WaitingCount int    // 待機中 + 呼出済み + 診察中（新しく来た人の前にいる人数）
	WaitMinutes  int
	Basis        string // actual | standard
	Reason       string // unavailable の理由: before_hours | closed | no_data
}

// WaitTimes は全診療科の待ち時間。個人を特定できる情報を含まない（REQ-007-AC-1）。
type WaitTimes struct {
	UpdatedAt   time.Time
	Departments []DepartmentWait
}

// Position は予約患者本人の順番。
type Position struct {
	Status          string // not_checked_in | waiting | called | in_consultation | done | canceled
	AheadCount      int
	EstimatedCallAt *time.Time
	Basis           string
}

// WaitTimeService は待ち時間の算出と、本人の順番・呼出見込みを担う。
// 待ち時間 = ceil(待ち人数 × 平均所要時間 ÷ 診察中の医師数)。
// 平均は直近 20 件の実績（3 件未満なら管理者設定の標準診察時間）。どちらも無ければ「算出不可」とし、数値を推測しない。
// IMP 15-clinic-reservation-wait-time-api TASK-0001 / REQ-004
type WaitTimeService interface {
	Get(ctx context.Context) (*WaitTimes, error)
	Position(ctx context.Context, patientID, appointmentID uuid.UUID) (*Position, error)
}

type waitTimeService struct {
	repos repository.Repositories
	now   func() time.Time

	mu       sync.Mutex // 算出中は他のリクエストを待たせ、DB アクセスを 1 回にまとめる
	cached   *WaitTimes
	cachedAt time.Time
}

func NewWaitTimeService(repos repository.Repositories, opts ...ClinicOption) WaitTimeService {
	return &waitTimeService{repos: repos, now: newClinicOptions(opts).now}
}

// estimate は診療科の算出に必要な前提（診察中の医師数と平均所要時間）。ok=false のときは reason が理由。
type estimate struct {
	ok     bool
	reason string
	basis  string
	lanes  int64
	avgSec int64
}

func (s *waitTimeService) estimate(ctx context.Context, dept db.Department, now time.Time) (estimate, error) {
	q := s.repos.Queue
	onDuty, err := q.CountOnDutyDoctors(ctx, db.CountOnDutyDoctorsParams{DepartmentID: dept.ID, At: tsOf(now)})
	if err != nil {
		return estimate{}, err
	}
	if onDuty == 0 {
		// 診療時間内なのに診察できる医師がいない = 臨時休診。時間外は開始前(この後に始まる)か終了後・休診日。
		scheduled, err := q.CountScheduledDoctors(ctx, db.CountScheduledDoctorsParams{DepartmentID: dept.ID, At: tsOf(now)})
		if err != nil {
			return estimate{}, err
		}
		if scheduled > 0 {
			return estimate{reason: WaitReasonClosed}, nil
		}
		later, err := q.CountDoctorsStartingLater(ctx, db.CountDoctorsStartingLaterParams{DepartmentID: dept.ID, At: tsOf(now)})
		if err != nil {
			return estimate{}, err
		}
		if later > 0 {
			return estimate{reason: WaitReasonBefore}, nil
		}
		return estimate{reason: WaitReasonClosed}, nil
	}

	samples, err := q.RecentConsultSeconds(ctx, db.RecentConsultSecondsParams{DepartmentID: dept.ID, ServiceDate: serviceDate(now), Limit: waitSampleLimit})
	if err != nil {
		return estimate{}, err
	}
	switch {
	case len(samples) >= waitMinSamples:
		var sum float64
		for _, v := range samples {
			sum += math.Min(v, waitSampleMaxSecs)
		}
		return estimate{ok: true, basis: WaitBasisActual, lanes: onDuty, avgSec: int64(math.Round(sum / float64(len(samples))))}, nil
	case dept.StandardConsultMinutes.Valid:
		return estimate{ok: true, basis: WaitBasisStandard, lanes: onDuty, avgSec: int64(dept.StandardConsultMinutes.Int32) * 60}, nil
	}
	return estimate{reason: WaitReasonNoData, lanes: onDuty}, nil
}

// minutesFor は n 人ぶんの待ち時間（分、切り上げ）。整数演算で浮動小数の誤差を避ける。
func (e estimate) minutesFor(n int64) int {
	denom := e.lanes * 60
	return int((n*e.avgSec + denom - 1) / denom)
}

func (s *waitTimeService) Get(ctx context.Context) (*WaitTimes, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.cached != nil && now.Sub(s.cachedAt) < waitCacheTTL {
		return s.copyCached(), nil
	}
	res, err := s.compute(ctx, now)
	if err != nil {
		return nil, err // 失敗はキャッシュしない
	}
	s.cached, s.cachedAt = res, now
	return s.copyCached(), nil
}

func (s *waitTimeService) copyCached() *WaitTimes {
	cp := *s.cached
	cp.Departments = append([]DepartmentWait(nil), s.cached.Departments...)
	return &cp
}

func (s *waitTimeService) compute(ctx context.Context, now time.Time) (*WaitTimes, error) {
	depts, err := s.repos.Admin.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	out := &WaitTimes{UpdatedAt: now, Departments: make([]DepartmentWait, 0, len(depts))}
	for _, d := range depts {
		w := DepartmentWait{DepartmentID: d.ID, Name: d.Name}
		est, err := s.estimate(ctx, d, now)
		if err != nil {
			return nil, err
		}
		if est.reason != "" && est.reason != WaitReasonNoData {
			w.Status, w.Reason = WaitStatusUnavail, est.reason
			out.Departments = append(out.Departments, w)
			continue
		}
		waiting, err := s.repos.Queue.CountWaiting(ctx, db.CountWaitingParams{DepartmentID: d.ID, ServiceDate: serviceDate(now)})
		if err != nil {
			return nil, err
		}
		w.WaitingCount = int(waiting)
		switch {
		case est.ok:
			w.Status, w.Basis, w.WaitMinutes = WaitStatusAvail, est.basis, est.minutesFor(waiting)
		case waiting == 0:
			// 誰も待っていなければ 0 分（事実）。根拠データが無いので basis は付けない
			w.Status = WaitStatusAvail
		default:
			// 待っている人がいるのにデータが無い → 推測せず算出不可
			w.Status, w.Reason, w.WaitingCount = WaitStatusUnavail, WaitReasonNoData, 0
		}
		out.Departments = append(out.Departments, w)
	}
	return out, nil
}

// Position: 予約患者本人の順番。他人の予約は ErrAppointmentNotFound。見込みを算出できないときは nil。
func (s *waitTimeService) Position(ctx context.Context, patientID, appointmentID uuid.UUID) (*Position, error) {
	a, err := s.repos.Appointments.GetAppointment(ctx, db.GetAppointmentParams{ID: appointmentID, PatientID: patientID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAppointmentNotFound
	}
	if err != nil {
		return nil, err
	}
	q, err := s.repos.Queue.GetQueueEntryForPatient(ctx, db.GetQueueEntryForPatientParams{AppointmentID: pgUUID(a.ID), PatientID: pgUUID(patientID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return &Position{Status: QueueNotCheckedIn}, nil
	}
	if err != nil {
		return nil, err
	}

	now := s.now()
	p := &Position{Status: q.Status}
	switch q.Status {
	case queueStatusCalled:
		p.EstimatedCallAt = &now // いま呼ばれている
	case queueStatusWaiting:
		ahead, err := s.repos.Queue.CountAhead(ctx, db.CountAheadParams{DepartmentID: q.DepartmentID, ServiceDate: q.ServiceDate, SortTime: q.SortTime})
		if err != nil {
			return nil, err
		}
		p.AheadCount = int(ahead)
		dept, err := s.repos.Admin.GetDepartment(ctx, q.DepartmentID)
		if err != nil {
			return nil, err
		}
		est, err := s.estimate(ctx, dept, now)
		if err != nil {
			return nil, err
		}
		if est.ok {
			at := now.Add(time.Duration(est.minutesFor(ahead)) * time.Minute).In(domain.JST)
			p.EstimatedCallAt, p.Basis = &at, est.basis
		}
	}
	return p, nil
}
