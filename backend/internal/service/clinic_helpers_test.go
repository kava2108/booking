package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/mocks"
)

// directTx はトランザクションを張らず、モックの Repositories をそのまま渡すテスト用 TxRunner。
type directTx struct{ repos repository.Repositories }

func (d directTx) WithinTx(_ context.Context, fn func(repository.Repositories) error) error {
	return fn(d.repos)
}

// clinicEnv は clinic-reservation の service テスト用の環境（全 repository がモック）。
type clinicEnv struct {
	slots *mocks.MockSlotRepository
	appts *mocks.MockAppointmentRepository
	queue *mocks.MockQueueRepository
	notif *mocks.MockNotificationRepository
	admin *mocks.MockAdminRepository
	users *mocks.MockUserRepository
	repos repository.Repositories
	tx    repository.TxRunner
	clock *time.Time
}

// 2026-10-12 は月曜。診療日の基準時刻（JST 9:00）。
var clinicNow = time.Date(2026, 10, 12, 9, 0, 0, 0, domain.JST)

func newClinicEnv(t *testing.T) *clinicEnv {
	t.Helper()
	clock := clinicNow
	e := &clinicEnv{
		slots: mocks.NewMockSlotRepository(t), appts: mocks.NewMockAppointmentRepository(t),
		queue: mocks.NewMockQueueRepository(t), notif: mocks.NewMockNotificationRepository(t),
		admin: mocks.NewMockAdminRepository(t), users: mocks.NewMockUserRepository(t), clock: &clock,
	}
	e.repos = repository.Repositories{Slots: e.slots, Appointments: e.appts, Queue: e.queue, Notifications: e.notif, Admin: e.admin}
	e.tx = directTx{e.repos}
	return e
}

func (e *clinicEnv) now() time.Time { return *e.clock }

func tsOf(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func clockOf(h, m int) pgtype.Time {
	return pgtype.Time{Microseconds: (int64(h)*3600 + int64(m)*60) * 1e6, Valid: true}
}

func puuid(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
