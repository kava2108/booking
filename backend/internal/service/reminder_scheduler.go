package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

const (
	defaultReminderHour  = 18 // 前日リマインドを出し始める時刻(JST)
	defaultNearThreshold = 3  // 「あと N 人」の N
)

// SchedulerResult は 1 回の実行で新しく登録した通知の件数。
type SchedulerResult struct{ Reminders, NearNotices int }

// ReminderScheduler は前日リマインドと「あと N 人」通知を、アウトボックスに冪等に登録する。
//   - リマインド: 前日 18:00(JST) 以降に、翌日の有効な予約へ 1 回。
//   - 順番接近: 待機中で、前にいる人数が N 以下になった患者(アカウントあり)へ 1 回。
//
// 重複登録は DB の部分ユニークインデックスで防ぐ（登録側が並行しても 1 件）。
// IMP 14-clinic-reservation-notification-outbox TASK-0001 / REQ-002-AC-6
type ReminderScheduler struct {
	repos        repository.Repositories
	now          func() time.Time
	reminderHour int
	threshold    int64
	log          zerolog.Logger
}

// SchedulerOption は ReminderScheduler の設定。
type SchedulerOption func(*ReminderScheduler)

func WithNearThreshold(n int) SchedulerOption {
	return func(s *ReminderScheduler) { s.threshold = int64(n) }
}
func WithSchedulerLogger(l zerolog.Logger) SchedulerOption {
	return func(s *ReminderScheduler) { s.log = l }
}

// NewReminderScheduler の opts には ClinicOption(時計)と SchedulerOption を混在して渡せる。
func NewReminderScheduler(repos repository.Repositories, opts ...any) *ReminderScheduler {
	s := &ReminderScheduler{repos: repos, now: time.Now, reminderHour: defaultReminderHour, threshold: defaultNearThreshold, log: zerolog.Nop()}
	var clinic []ClinicOption
	for _, o := range opts {
		switch v := o.(type) {
		case ClinicOption:
			clinic = append(clinic, v)
		case SchedulerOption:
			v(s)
		}
	}
	s.now = newClinicOptions(clinic).now
	return s
}

// RunOnce はリマインドと順番接近を 1 回ずつ処理する。片方が失敗してももう片方は実行し、エラーはまとめて返す。
func (s *ReminderScheduler) RunOnce(ctx context.Context) (SchedulerResult, error) {
	var res SchedulerResult
	var errs []error
	var err error
	if res.Reminders, err = s.enqueueReminders(ctx); err != nil {
		errs = append(errs, err)
	}
	if res.NearNotices, err = s.enqueueNearNotices(ctx); err != nil {
		errs = append(errs, err)
	}
	return res, errors.Join(errs...)
}

func (s *ReminderScheduler) enqueueReminders(ctx context.Context) (int, error) {
	now := s.now()
	if now.In(domain.JST).Hour() < s.reminderHour {
		return 0, nil
	}
	tomorrow := domain.StartOfDay(now).AddDate(0, 0, 1)
	rows, err := s.repos.Appointments.ListAppointmentsNeedingReminder(ctx, db.ListAppointmentsNeedingReminderParams{
		FromAt: tsOf(tomorrow), ToAt: tsOf(tomorrow.Add(24 * time.Hour)),
	})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range rows {
		payload, err := json.Marshal(domain.AppointmentNotice{
			AppointmentID: r.ID.String(), AppointmentNo: r.AppointmentNo, StartAt: r.StartAt.Time.In(domain.JST),
			Department: r.DepartmentName, Doctor: r.DoctorName,
		})
		if err != nil {
			return count, err
		}
		n, err := s.repos.Notifications.EnqueueNotificationOnce(ctx, db.EnqueueNotificationOnceParams{UserID: r.PatientID, Kind: domain.KindAppointmentReminder, Payload: payload})
		if err != nil {
			return count, err
		}
		count += int(n)
	}
	return count, nil
}

func (s *ReminderScheduler) enqueueNearNotices(ctx context.Context) (int, error) {
	rows, err := s.repos.Queue.ListQueueEntriesNeedingNearNotice(ctx, db.ListQueueEntriesNeedingNearNoticeParams{
		ServiceDate: serviceDate(s.now()), MaxAhead: s.threshold,
	})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range rows {
		payload, err := json.Marshal(domain.QueueNearNotice{QueueEntryID: r.ID.String(), TicketNo: int(r.TicketNo), Department: r.DepartmentName, AheadCount: int(r.Ahead)})
		if err != nil {
			return count, err
		}
		n, err := s.repos.Notifications.EnqueueNotificationOnce(ctx, db.EnqueueNotificationOnceParams{UserID: r.PatientID.Bytes, Kind: domain.KindQueueNear, Payload: payload})
		if err != nil {
			return count, err
		}
		count += int(n)
	}
	return count, nil
}

// Run は interval ごとに RunOnce を実行する。ctx が終了するまでブロックする。
func (s *ReminderScheduler) Run(ctx context.Context, interval time.Duration) {
	run := func() {
		res, err := s.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			s.log.Error().Err(err).Msg("reminder scheduler iteration failed")
		}
		if res.Reminders+res.NearNotices > 0 {
			s.log.Info().Int("reminders", res.Reminders).Int("near_notices", res.NearNotices).Msg("notifications enqueued")
		}
	}
	run()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
