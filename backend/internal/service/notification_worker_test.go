package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/repository/mocks"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 14-clinic-reservation-notification-outbox TASK-0001: 通知ワーカー（at-least-once・バックオフ・最大 5 回）

type sentMail struct{ to, subject, body string }

type fakeSender struct {
	mu   sync.Mutex
	sent []sentMail
	err  func(to string) error
}

func (f *fakeSender) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		if err := f.err(to); err != nil {
			return err
		}
	}
	f.sent = append(f.sent, sentMail{to, subject, body})
	return nil
}

type workerEnv struct {
	notifs *mocks.MockNotificationRepository
	users  *mocks.MockUserRepository
	sender *fakeSender
	logs   *bytes.Buffer
	w      *service.NotificationWorker
}

func newWorker(t *testing.T) *workerEnv {
	t.Helper()
	e := &workerEnv{notifs: mocks.NewMockNotificationRepository(t), users: mocks.NewMockUserRepository(t), sender: &fakeSender{}, logs: &bytes.Buffer{}}
	e.w = service.NewNotificationWorker(e.notifs, e.users, e.sender, service.WithWorkerLogger(zerolog.New(e.logs)))
	return e
}

func notif(kind string, attempts int32, payload []byte) db.Notification {
	return db.Notification{ID: uuid.New(), UserID: patient, Kind: kind, Payload: payload, Status: "pending", Attempts: attempts}
}

func confirmedPayload(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(domain.AppointmentNotice{AppointmentID: "a", AppointmentNo: "A-1", StartAt: day(19, 9, 30), Department: "内科", Doctor: "山田"})
	require.NoError(t, err)
	return b
}

func (e *workerEnv) expectClaim(rows ...db.Notification) {
	e.notifs.EXPECT().ClaimNotifications(mock.Anything, db.ClaimNotificationsParams{Limit: 20, LeaseSeconds: 300}).Return(rows, nil)
}

func (e *workerEnv) expectUser(email, name string) {
	e.users.EXPECT().FindByID(mock.Anything, patient).Return(&domain.User{ID: patient, Email: email, Name: name}, nil)
}

func TestWorker_SendsAndMarksSent(t *testing.T) {
	e := newWorker(t)
	n := notif(domain.KindAppointmentConfirmed, 1, confirmedPayload(t))
	e.expectClaim(n)
	e.expectUser("taro@example.com", "山田 太郎")
	e.notifs.EXPECT().MarkNotificationSent(mock.Anything, n.ID).Return(nil)

	count, err := e.w.RunOnce(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, count)
	require.Len(t, e.sender.sent, 1)
	assert.Equal(t, "taro@example.com", e.sender.sent[0].to)
	assert.Contains(t, e.sender.sent[0].body, "A-1")
	assert.Contains(t, e.sender.sent[0].body, "山田 太郎 様")
}

func TestWorker_NothingToDo(t *testing.T) {
	e := newWorker(t)
	e.expectClaim()

	count, err := e.w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
	assert.Empty(t, e.sender.sent)
}

// 送信失敗: 試行回数に応じて指数バックオフで再スケジュール（1, 2, 4, 8 分）
func TestWorker_RetriesWithExponentialBackoff(t *testing.T) {
	for attempts, wantDelay := range map[int32]float64{1: 60, 2: 120, 3: 240, 4: 480} {
		t.Run(time.Duration(wantDelay*float64(time.Second)).String(), func(t *testing.T) {
			e := newWorker(t)
			e.sender.err = func(string) error { return errors.New("smtp down") }
			n := notif(domain.KindAppointmentConfirmed, attempts, confirmedPayload(t))
			e.expectClaim(n)
			e.expectUser("a@example.com", "n")
			e.notifs.EXPECT().RescheduleNotification(mock.Anything, db.RescheduleNotificationParams{ID: n.ID, DelaySeconds: wantDelay}).Return(nil)
			// MarkNotificationSent / Failed は呼ばれない

			_, err := e.w.RunOnce(context.Background())
			require.NoError(t, err, "個別の送信失敗は RunOnce のエラーにしない")
		})
	}
}

func TestWorker_BackoffIsCapped(t *testing.T) {
	e := service.NewNotificationWorker(nil, nil, nil, service.WithWorkerMaxAttempts(100))
	assert.Equal(t, 3600*time.Second, e.BackoffFor(12), "上限 1 時間")
	assert.Equal(t, time.Minute, e.BackoffFor(1))
}

// 最大試行回数(5)に達したら failed
func TestWorker_MarksFailedAtMaxAttempts(t *testing.T) {
	e := newWorker(t)
	e.sender.err = func(string) error { return errors.New("smtp down") }
	n := notif(domain.KindAppointmentConfirmed, 5, confirmedPayload(t))
	e.expectClaim(n)
	e.expectUser("a@example.com", "n")
	e.notifs.EXPECT().MarkNotificationFailed(mock.Anything, n.ID).Return(nil)

	_, err := e.w.RunOnce(context.Background())
	require.NoError(t, err)
}

// 再試行しても無駄なものは即 failed（宛先不明・未知の種別・壊れた payload）
func TestWorker_PermanentFailuresAreMarkedFailedWithoutSending(t *testing.T) {
	t.Run("宛先ユーザーがいない", func(t *testing.T) {
		e := newWorker(t)
		n := notif(domain.KindAppointmentConfirmed, 1, confirmedPayload(t))
		e.expectClaim(n)
		e.users.EXPECT().FindByID(mock.Anything, patient).Return(nil, repository.ErrNotFound)
		e.notifs.EXPECT().MarkNotificationFailed(mock.Anything, n.ID).Return(nil)
		_, err := e.w.RunOnce(context.Background())
		require.NoError(t, err)
		assert.Empty(t, e.sender.sent)
	})
	t.Run("未知の種別", func(t *testing.T) {
		e := newWorker(t)
		n := notif("mystery", 1, []byte(`{}`))
		e.expectClaim(n)
		e.expectUser("a@example.com", "n")
		e.notifs.EXPECT().MarkNotificationFailed(mock.Anything, n.ID).Return(nil)
		_, _ = e.w.RunOnce(context.Background())
		assert.Empty(t, e.sender.sent)
	})
	t.Run("壊れた payload", func(t *testing.T) {
		e := newWorker(t)
		n := notif(domain.KindAppointmentConfirmed, 1, []byte(`{bad`))
		e.expectClaim(n)
		e.expectUser("a@example.com", "n")
		e.notifs.EXPECT().MarkNotificationFailed(mock.Anything, n.ID).Return(nil)
		_, _ = e.w.RunOnce(context.Background())
		assert.Empty(t, e.sender.sent)
	})
}

// ユーザー取得の一時的な障害は送信失敗と同じく再試行する（failed にしない）
func TestWorker_TransientLookupErrorIsRetried(t *testing.T) {
	e := newWorker(t)
	n := notif(domain.KindAppointmentConfirmed, 2, confirmedPayload(t))
	e.expectClaim(n)
	e.users.EXPECT().FindByID(mock.Anything, patient).Return(nil, errors.New("db down"))
	e.notifs.EXPECT().RescheduleNotification(mock.Anything, db.RescheduleNotificationParams{ID: n.ID, DelaySeconds: 120}).Return(nil)

	_, err := e.w.RunOnce(context.Background())
	require.NoError(t, err)
}

// 1 件の失敗が他の通知の送信を止めない
func TestWorker_OneFailureDoesNotBlockOthers(t *testing.T) {
	e := newWorker(t)
	bad, good := notif(domain.KindAppointmentConfirmed, 1, confirmedPayload(t)), notif(domain.KindAppointmentConfirmed, 1, confirmedPayload(t))
	bad.UserID = uuid.New()
	e.expectClaim(bad, good)
	e.users.EXPECT().FindByID(mock.Anything, bad.UserID).Return(&domain.User{Email: "bad@example.com", Name: "x"}, nil)
	e.users.EXPECT().FindByID(mock.Anything, patient).Return(&domain.User{Email: "good@example.com", Name: "y"}, nil)
	e.sender.err = func(to string) error {
		if to == "bad@example.com" {
			return errors.New("rejected")
		}
		return nil
	}
	e.notifs.EXPECT().RescheduleNotification(mock.Anything, mock.Anything).Return(nil).Once()
	e.notifs.EXPECT().MarkNotificationSent(mock.Anything, good.ID).Return(nil)

	count, err := e.w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	require.Len(t, e.sender.sent, 1)
	assert.Equal(t, "good@example.com", e.sender.sent[0].to)
}

// 状態更新の失敗は他の通知の処理を止めない（at-least-once: リース切れで再取得される）
func TestWorker_MarkErrorIsLoggedAndContinues(t *testing.T) {
	e := newWorker(t)
	a, b := notif(domain.KindAppointmentConfirmed, 1, confirmedPayload(t)), notif(domain.KindAppointmentConfirmed, 1, confirmedPayload(t))
	e.expectClaim(a, b)
	e.users.EXPECT().FindByID(mock.Anything, patient).Return(&domain.User{Email: "a@example.com", Name: "n"}, nil).Times(2)
	e.notifs.EXPECT().MarkNotificationSent(mock.Anything, a.ID).Return(errors.New("db down"))
	e.notifs.EXPECT().MarkNotificationSent(mock.Anything, b.ID).Return(nil)

	_, err := e.w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Len(t, e.sender.sent, 2)
}

func TestWorker_ClaimErrorIsReturned(t *testing.T) {
	e := newWorker(t)
	boom := errors.New("db down")
	e.notifs.EXPECT().ClaimNotifications(mock.Anything, mock.Anything).Return(nil, boom)

	_, err := e.w.RunOnce(context.Background())
	assert.ErrorIs(t, err, boom)
}

// 個人情報保護: ログに宛先メール・氏名・本文・payload を出さない
func TestWorker_LogsDoNotContainPersonalData(t *testing.T) {
	e := newWorker(t)
	e.sender.err = func(string) error { return errors.New("smtp down") }
	n := notif(domain.KindAppointmentConfirmed, 5, confirmedPayload(t))
	e.expectClaim(n)
	e.expectUser("private.person@example.com", "秘密 花子")
	e.notifs.EXPECT().MarkNotificationFailed(mock.Anything, n.ID).Return(nil)

	_, _ = e.w.RunOnce(context.Background())

	logs := e.logs.String()
	require.NotEmpty(t, logs, "失敗はログに残る")
	for _, leaked := range []string{"private.person@example.com", "秘密 花子", "A-1", "内科"} {
		assert.NotContains(t, logs, leaked)
	}
	assert.Contains(t, logs, n.ID.String(), "通知 ID で追跡できる")
}

// Run: 即時に 1 回、その後は定期的に実行し、ctx で停止する
func TestWorker_RunLoop(t *testing.T) {
	notifs := mocks.NewMockNotificationRepository(t)
	var mu sync.Mutex
	calls := 0
	notifs.EXPECT().ClaimNotifications(mock.Anything, mock.Anything).RunAndReturn(func(context.Context, db.ClaimNotificationsParams) ([]db.Notification, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return nil, errors.New("transient") // エラーでもループは継続する
	})
	w := service.NewNotificationWorker(notifs, mocks.NewMockUserRepository(t), &fakeSender{}, service.WithWorkerLogger(zerolog.Nop()))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx, 10*time.Millisecond); close(done) }()

	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return calls >= 3 }, 2*time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ctx のキャンセルで停止しない")
	}
}
