package service

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
)

// NotificationSender は通知の送信手段。初期は SMTP。SMS/LINE を追加するときはこの interface を実装する。
type NotificationSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

const (
	defaultWorkerBatch       = 20
	defaultWorkerLease       = 5 * time.Minute
	defaultWorkerMaxAttempts = 5
	defaultWorkerBackoff     = time.Minute
	maxBackoff               = time.Hour
)

// NotificationWorker はアウトボックス(notifications)の通知を送信する。
//   - 取得はリース方式(FOR UPDATE SKIP LOCKED)。複数ワーカーでも重複しない。長い Tx を保持しない。
//   - 送信は at-least-once（送信後・状態更新前にクラッシュすると再送され得る）。
//   - 失敗は指数バックオフ(1,2,4,8 分…上限 1 時間)で再試行し、最大試行回数で failed にする。
//   - 宛先ユーザー不明・未知の種別・壊れた payload は再試行しても無駄なので即 failed。
//   - ログには通知 ID と種別のみを出す（宛先・氏名・本文・payload は出さない）。
//
// IMP 14-clinic-reservation-notification-outbox TASK-0001
type NotificationWorker struct {
	notifs      repository.NotificationRepository
	users       repository.UserRepository
	sender      NotificationSender
	batch       int32
	lease       time.Duration
	maxAttempts int32
	backoff     time.Duration
	log         zerolog.Logger
}

// WorkerOption は NotificationWorker の設定。
type WorkerOption func(*NotificationWorker)

func WithWorkerLogger(l zerolog.Logger) WorkerOption {
	return func(w *NotificationWorker) { w.log = l }
}
func WithWorkerBatch(n int) WorkerOption { return func(w *NotificationWorker) { w.batch = int32(n) } }
func WithWorkerLease(d time.Duration) WorkerOption {
	return func(w *NotificationWorker) { w.lease = d }
}
func WithWorkerMaxAttempts(n int) WorkerOption {
	return func(w *NotificationWorker) { w.maxAttempts = int32(n) }
}
func WithWorkerBackoff(base time.Duration) WorkerOption {
	return func(w *NotificationWorker) { w.backoff = base }
}

func NewNotificationWorker(notifs repository.NotificationRepository, users repository.UserRepository, sender NotificationSender, opts ...WorkerOption) *NotificationWorker {
	w := &NotificationWorker{
		notifs: notifs, users: users, sender: sender,
		batch: defaultWorkerBatch, lease: defaultWorkerLease, maxAttempts: defaultWorkerMaxAttempts, backoff: defaultWorkerBackoff,
		log: zerolog.Nop(),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// BackoffFor は attempts 回目の失敗後の待ち時間（base × 2^(attempts-1)、上限 1 時間）。
func (w *NotificationWorker) BackoffFor(attempts int32) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 20 {
		return maxBackoff
	}
	d := w.backoff << (attempts - 1)
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

// RunOnce は送信対象を 1 バッチ処理し、取得した件数を返す。個々の送信失敗はエラーにしない。
func (w *NotificationWorker) RunOnce(ctx context.Context) (int, error) {
	claimed, err := w.notifs.ClaimNotifications(ctx, db.ClaimNotificationsParams{Limit: w.batch, LeaseSeconds: w.lease.Seconds()})
	if err != nil {
		return 0, err
	}
	for _, n := range claimed {
		w.process(ctx, n)
	}
	return len(claimed), nil
}

func (w *NotificationWorker) process(ctx context.Context, n db.Notification) {
	log := w.log.With().Str("notification_id", n.ID.String()).Str("kind", n.Kind).Int32("attempts", n.Attempts).Logger()

	user, err := w.users.FindByID(ctx, n.UserID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		log.Warn().Msg("recipient not found; giving up")
		w.fail(ctx, n, log)
		return
	case err != nil:
		log.Error().Err(err).Msg("recipient lookup failed")
		w.retryOrFail(ctx, n, log)
		return
	}

	mail, err := RenderNotification(n.Kind, user.Name, n.Payload)
	if err != nil {
		log.Error().Err(err).Msg("cannot render notification; giving up")
		w.fail(ctx, n, log)
		return
	}
	if err := w.sender.Send(ctx, user.Email, mail.Subject, mail.Body); err != nil {
		log.Warn().Err(err).Msg("send failed")
		w.retryOrFail(ctx, n, log)
		return
	}
	if err := w.notifs.MarkNotificationSent(ctx, n.ID); err != nil {
		// 送信済みだが状態を更新できなかった。リース切れで再取得され再送され得る(at-least-once)。
		log.Error().Err(err).Msg("sent but could not mark as sent")
	}
}

func (w *NotificationWorker) retryOrFail(ctx context.Context, n db.Notification, log zerolog.Logger) {
	if n.Attempts >= w.maxAttempts {
		log.Error().Msg("max attempts reached; marking as failed")
		w.fail(ctx, n, log)
		return
	}
	delay := w.BackoffFor(n.Attempts)
	if err := w.notifs.RescheduleNotification(ctx, db.RescheduleNotificationParams{ID: n.ID, DelaySeconds: delay.Seconds()}); err != nil {
		log.Error().Err(err).Msg("could not reschedule")
	}
}

func (w *NotificationWorker) fail(ctx context.Context, n db.Notification, log zerolog.Logger) {
	if err := w.notifs.MarkNotificationFailed(ctx, n.ID); err != nil {
		log.Error().Err(err).Msg("could not mark as failed")
	}
}

// Run は interval ごとに RunOnce を実行する。ctx が終了するまでブロックする。エラーはログに出して継続する。
func (w *NotificationWorker) Run(ctx context.Context, interval time.Duration) {
	run := func() {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			w.log.Error().Err(err).Msg("notification worker iteration failed")
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
