package repository_test

import (
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/mocks"
)

// mockery 生成モックが interface を満たし、service のユニットテストで使えること。
var (
	_ repository.SlotRepository         = (*mocks.MockSlotRepository)(nil)
	_ repository.AppointmentRepository  = (*mocks.MockAppointmentRepository)(nil)
	_ repository.QueueRepository        = (*mocks.MockQueueRepository)(nil)
	_ repository.NotificationRepository = (*mocks.MockNotificationRepository)(nil)
	_ repository.AdminRepository        = (*mocks.MockAdminRepository)(nil)
	_ repository.TxRunner               = (*mocks.MockTxRunner)(nil)
)

func TestMocksUsable(t *testing.T) {
	m := mocks.NewMockSlotRepository(t)
	m.EXPECT().ReleaseSlot(mock.Anything, mock.Anything).Return(nil)

	if err := m.ReleaseSlot(t.Context(), [16]byte{}); err != nil {
		t.Fatal(err)
	}
}
