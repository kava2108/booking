package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// IMP 14-clinic-reservation-notification-outbox TASK-0001: 実 DB を使った通知の通しテスト

type notifEnv struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	store  *repository.Store
	users  repository.UserRepository
	appts  service.AppointmentService
	queue  service.QueueService
	sender *fakeSender
	deptID int64
	docID  int64
}

func newNotifEnv(t *testing.T) *notifEnv {
	t.Helper()
	pool := pgtest.NewPool(t)
	store := repository.NewStore(pool)
	clock := func() time.Time { return clinicNow }
	e := &notifEnv{
		ctx: context.Background(), pool: pool, store: store, users: repository.NewUserRepository(db.New(pool)), sender: &fakeSender{},
		appts: service.NewAppointmentService(store.Repositories(), store, service.WithClinicClock(clock)),
		queue: service.NewQueueService(store.Repositories(), store, service.WithClinicClock(clock)),
	}
	require.NoError(t, pool.QueryRow(e.ctx, `INSERT INTO departments(name) VALUES ('内科') RETURNING id`).Scan(&e.deptID))
	require.NoError(t, pool.QueryRow(e.ctx, `INSERT INTO doctors(department_id,name) VALUES ($1,'山田') RETURNING id`, e.deptID).Scan(&e.docID))
	return e
}

func (e *notifEnv) user(t *testing.T, email string) uuid.UUID {
	t.Helper()
	u, err := e.users.Create(e.ctx, repository.CreateUserParams{Email: email, PasswordHash: "h", Name: "患者 " + email})
	require.NoError(t, err)
	return u.ID
}

func (e *notifEnv) slot(t *testing.T, start time.Time, capacity int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.pool.QueryRow(e.ctx, `INSERT INTO slots(department_id,doctor_id,start_at,capacity) VALUES ($1,$2,$3,$4) RETURNING id`, e.deptID, e.docID, start, capacity).Scan(&id))
	return id
}

func (e *notifEnv) worker(opts ...service.WorkerOption) *service.NotificationWorker {
	repos := e.store.Repositories()
	return service.NewNotificationWorker(repos.Notifications, e.users, e.sender, append([]service.WorkerOption{service.WithWorkerLogger(zerolog.Nop())}, opts...)...)
}

func (e *notifEnv) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(e.ctx, q, args...).Scan(&n))
	return n
}

func (e *notifEnv) makeDue(t *testing.T) {
	t.Helper()
	_, err := e.pool.Exec(e.ctx, `UPDATE notifications SET send_after = now() - interval '1 second' WHERE status = 'pending'`)
	require.NoError(t, err)
}

// 予約確定 → 同一 Tx で登録された通知 → ワーカーが送信 → sent。再実行しても二重送信しない
func TestNotificationFlow_BookingToDelivery(t *testing.T) {
	e := newNotifEnv(t)
	uid := e.user(t, "taro@example.com")
	res, err := e.appts.Create(e.ctx, uid, service.CreateAppointmentInput{SlotID: e.slot(t, day(19, 9, 30), 1), VisitType: "first", SymptomNote: "糖尿病の疑い"})
	require.NoError(t, err)

	n, err := e.worker().RunOnce(e.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	require.Len(t, e.sender.sent, 1)
	mail := e.sender.sent[0]
	assert.Equal(t, "taro@example.com", mail.to)
	assert.Contains(t, mail.body, res.AppointmentNo)
	assert.Contains(t, mail.body, "2026年10月19日(月) 09:30")
	assert.NotContains(t, mail.subject+mail.body, "糖尿病", "症状メモはメールに出ない (REQ-007-AC-4)")
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM notifications WHERE status='sent'`))

	n, err = e.worker().RunOnce(e.ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "送信済みは再送しない")
	assert.Len(t, e.sender.sent, 1)
}

// 送信失敗: pending のまま後ろ倒し → 5 回目の失敗で failed
func TestNotificationFlow_RetriesThenFails(t *testing.T) {
	e := newNotifEnv(t)
	uid := e.user(t, "taro@example.com")
	_, err := e.appts.Create(e.ctx, uid, service.CreateAppointmentInput{SlotID: e.slot(t, day(19, 9, 30), 1), VisitType: "first"})
	require.NoError(t, err)
	e.sender.err = func(string) error { return errors.New("smtp down") }
	w := e.worker()

	for attempt := 1; attempt <= 4; attempt++ {
		n, err := w.RunOnce(e.ctx)
		require.NoError(t, err)
		require.Equal(t, 1, n, "試行 %d", attempt)

		var status string
		var attempts int
		var delay float64
		require.NoError(t, e.pool.QueryRow(e.ctx, `SELECT status, attempts, EXTRACT(EPOCH FROM (send_after - now())) FROM notifications`).Scan(&status, &attempts, &delay))
		assert.Equal(t, "pending", status)
		assert.Equal(t, attempt, attempts)
		assert.InDelta(t, 60*float64(int(1)<<(attempt-1)), delay, 5, "指数バックオフ")

		n, _ = w.RunOnce(e.ctx)
		assert.Zero(t, n, "バックオフ中は取得されない")
		e.makeDue(t)
	}

	n, err := w.RunOnce(e.ctx) // 5 回目
	require.NoError(t, err)
	require.Equal(t, 1, n)
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM notifications WHERE status='failed' AND attempts=5`))
	assert.Empty(t, e.sender.sent)
}

// 送信が一度失敗しても、回復すれば送られる
func TestNotificationFlow_RecoversAfterTransientFailure(t *testing.T) {
	e := newNotifEnv(t)
	uid := e.user(t, "taro@example.com")
	_, err := e.appts.Create(e.ctx, uid, service.CreateAppointmentInput{SlotID: e.slot(t, day(19, 9, 30), 1), VisitType: "first"})
	require.NoError(t, err)
	fail := true
	e.sender.err = func(string) error {
		if fail {
			return errors.New("smtp down")
		}
		return nil
	}
	w := e.worker()

	_, _ = w.RunOnce(e.ctx)
	assert.Empty(t, e.sender.sent)
	fail = false
	e.makeDue(t)
	_, _ = w.RunOnce(e.ctx)

	assert.Len(t, e.sender.sent, 1)
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM notifications WHERE status='sent' AND attempts=2`))
}

// 複数ワーカーが同時に動いても、各通知は 1 回だけ送られる
func TestNotificationFlow_ConcurrentWorkersSendEachOnce(t *testing.T) {
	e := newNotifEnv(t)
	for i := 0; i < 30; i++ {
		uid := e.user(t, fmt.Sprintf("u%d@example.com", i))
		_, err := e.store.Repositories().Notifications.EnqueueNotification(e.ctx, db.EnqueueNotificationParams{
			UserID: uid, Kind: domain.KindAppointmentConfirmed,
			Payload: []byte(`{"appointment_id":"a","appointment_no":"A-1","start_at":"2026-10-19T09:30:00+09:00","department":"内科","doctor":"山田"}`),
		})
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 4; i++ {
				if _, err := e.worker(service.WithWorkerBatch(7)).RunOnce(e.ctx); err != nil {
					t.Errorf("worker: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	counts := map[string]int{}
	for _, m := range e.sender.sent {
		counts[m.to]++
	}
	assert.Len(t, counts, 30, "全員に届く")
	for to, c := range counts {
		assert.Equal(t, 1, c, "%s に重複送信", to)
	}
	assert.Equal(t, 30, e.count(t, `SELECT count(*) FROM notifications WHERE status='sent'`))
}

// 取消・日時変更の通知も同じ経路で届く
func TestNotificationFlow_CancelAndRescheduleMails(t *testing.T) {
	e := newNotifEnv(t)
	uid := e.user(t, "taro@example.com")
	oldSlot, newSlot := e.slot(t, day(19, 9, 30), 1), e.slot(t, day(26, 10, 0), 1)
	res, err := e.appts.Create(e.ctx, uid, service.CreateAppointmentInput{SlotID: oldSlot, VisitType: "first"})
	require.NoError(t, err)
	_, err = e.appts.Reschedule(e.ctx, uid, res.ID, newSlot)
	require.NoError(t, err)
	require.NoError(t, e.appts.Cancel(e.ctx, uid, res.ID))

	n, err := e.worker().RunOnce(e.ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	var subjects []string
	for _, m := range e.sender.sent {
		subjects = append(subjects, m.subject)
	}
	joined := strings.Join(subjects, "|")
	assert.Contains(t, joined, "予約確認")
	assert.Contains(t, joined, "予約変更")
	assert.Contains(t, joined, "キャンセル")
}

// ---------- 前日リマインド / あと N 人 (REQ-002-AC-6) ----------

func (e *notifEnv) scheduler(now time.Time) *service.ReminderScheduler {
	return service.NewReminderScheduler(e.store.Repositories(), service.WithClinicClock(func() time.Time { return now }), service.WithSchedulerLogger(zerolog.Nop()))
}

func TestReminderFlow_OncePerAppointmentEvenWithConcurrentSchedulers(t *testing.T) {
	e := newNotifEnv(t)
	uid := e.user(t, "taro@example.com")
	_, err := e.appts.Create(e.ctx, uid, service.CreateAppointmentInput{SlotID: e.slot(t, day(13, 9, 30), 1), VisitType: "first"}) // 明日
	require.NoError(t, err)
	other := e.user(t, "other@example.com")
	_, err = e.appts.Create(e.ctx, other, service.CreateAppointmentInput{SlotID: e.slot(t, day(19, 9, 30), 1), VisitType: "first"}) // 来週（対象外）
	require.NoError(t, err)

	before := e.scheduler(day(12, 17, 59))
	res, err := before.RunOnce(e.ctx)
	require.NoError(t, err)
	assert.Zero(t, res.Reminders, "18:00 前は登録しない")

	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := e.scheduler(day(12, 18, 0)).RunOnce(e.ctx)
			if err != nil {
				t.Errorf("scheduler: %v", err)
			}
			mu.Lock()
			total += r.Reminders
			mu.Unlock()
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, total, "並行実行しても 1 件だけ登録される")
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM notifications WHERE kind='appointment_reminder'`))

	// 後続の実行では登録されず、ワーカーが「明日」のメールを送る
	res, _ = e.scheduler(day(12, 19, 0)).RunOnce(e.ctx)
	assert.Zero(t, res.Reminders)
	_, err = e.worker().RunOnce(e.ctx)
	require.NoError(t, err)
	var reminder *sentMail
	for i := range e.sender.sent {
		if strings.Contains(e.sender.sent[i].subject, "明日") {
			reminder = &e.sender.sent[i]
		}
	}
	require.NotNil(t, reminder)
	assert.Equal(t, "taro@example.com", reminder.to)
	assert.Contains(t, reminder.body, "2026年10月13日(火) 09:30")
}

func TestQueueNearFlow_NotifiesOnceWhenFewAhead(t *testing.T) {
	e := newNotifEnv(t)
	for i := 0; i < 4; i++ {
		_, err := e.queue.CreateWalkIn(e.ctx, service.WalkInInput{DepartmentID: e.deptID, DisplayName: fmt.Sprintf("先頭%d", i)})
		require.NoError(t, err)
	}
	uid := e.user(t, "waiting@example.com")
	// 後から受付した患者（前に 4 人）。受付時刻を進めて順序を明確にする
	later := clinicNow.Add(time.Minute)
	repos := e.store.Repositories()
	q, err := repos.Queue.EnqueueEntry(e.ctx, db.EnqueueEntryParams{
		DepartmentID: e.deptID, PatientID: puuid(uid), Source: "walk_in", ServiceDate: serviceDateOf(later), TicketNo: 5, SortTime: tsOf(later), CheckedInAt: tsOf(later),
	})
	require.NoError(t, err)

	sched := e.scheduler(later)
	res, err := sched.RunOnce(e.ctx)
	require.NoError(t, err)
	assert.Zero(t, res.NearNotices, "前に 4 人いる間は通知しない（N=3）")

	// 先頭の 1 人が診察を終えると「あと 3 人」
	list, err := repos.Queue.ListStaffQueue(e.ctx, db.ListStaffQueueParams{DepartmentID: e.deptID, ServiceDate: serviceDateOf(later)})
	require.NoError(t, err)
	_, err = e.queue.Call(e.ctx, list[0].ID)
	require.NoError(t, err)
	_, err = e.queue.Start(e.ctx, list[0].ID)
	require.NoError(t, err)
	_, err = e.queue.Complete(e.ctx, list[0].ID)
	require.NoError(t, err)

	res, err = sched.RunOnce(e.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, res.NearNotices)
	res, _ = sched.RunOnce(e.ctx)
	assert.Zero(t, res.NearNotices, "同じ項目には 2 回通知しない")

	_, err = e.worker().RunOnce(e.ctx)
	require.NoError(t, err)
	require.Len(t, e.sender.sent, 1)
	assert.Equal(t, "waiting@example.com", e.sender.sent[0].to)
	assert.Contains(t, e.sender.sent[0].body, "あと3人")
	assert.Contains(t, e.sender.sent[0].body, "5", "受付番号")
	_ = q
}

func serviceDateOf(t time.Time) pgtype.Date {
	return pgtype.Date{Time: domain.StartOfDay(t), Valid: true}
}
