// Package repository は service 層が依存する DB アクセスの interface を定義する。
// 実装は sqlc 生成の *db.Queries をそのまま使う（下のコンパイル時チェック参照）。
// service は *db.Queries を直接受け取らず、必ずここの interface を介して DI する。
// IMP 5-clinic-reservation-sqlc-repository TASK-0001
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kava2108/booking/backend/internal/repository/db"
)

// SlotRepository は予約枠の参照・排他確保・解放を扱う。
type SlotRepository interface {
	ListSlots(ctx context.Context, arg db.ListSlotsParams) ([]db.Slot, error)
	GetSlot(ctx context.Context, id uuid.UUID) (db.Slot, error)
	GetSlotDetail(ctx context.Context, id uuid.UUID) (db.GetSlotDetailRow, error)
	CreateSlotIfAbsent(ctx context.Context, arg db.CreateSlotIfAbsentParams) error
	// ReserveSlot は満席・休診のとき pgx.ErrNoRows を返す。
	ReserveSlot(ctx context.Context, id uuid.UUID) (db.Slot, error)
	ReleaseSlot(ctx context.Context, id uuid.UUID) error
	CloseSlots(ctx context.Context, arg db.CloseSlotsParams) ([]uuid.UUID, error)
}

// AppointmentRepository は予約を扱う。患者向けメソッドは patient_id で絞る。
type AppointmentRepository interface {
	CreateAppointment(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error)
	GetAppointment(ctx context.Context, arg db.GetAppointmentParams) (db.Appointment, error)
	ListAppointmentsByPatient(ctx context.Context, patientID uuid.UUID) ([]db.ListAppointmentsByPatientRow, error)
	CancelAppointment(ctx context.Context, arg db.CancelAppointmentParams) (db.Appointment, error)
	RescheduleAppointment(ctx context.Context, arg db.RescheduleAppointmentParams) (db.Appointment, error)
	// GetAppointmentWithSlot は患者スコープなし。スタッフ操作（来院受付）専用。
	GetAppointmentWithSlot(ctx context.Context, id uuid.UUID) (db.GetAppointmentWithSlotRow, error)
	CompleteAppointment(ctx context.Context, id uuid.UUID) (int64, error)
	// ListAppointmentsForStaff は氏名を含む。スタッフ操作専用。
	ListAppointmentsForStaff(ctx context.Context, arg db.ListAppointmentsForStaffParams) ([]db.ListAppointmentsForStaffRow, error)
	ListAppointmentsNeedingReminder(ctx context.Context, arg db.ListAppointmentsNeedingReminderParams) ([]db.ListAppointmentsNeedingReminderRow, error)
	CountPatientBookingsAt(ctx context.Context, arg db.CountPatientBookingsAtParams) (int64, error)
	ListBookedAppointmentsForNotice(ctx context.Context, arg db.ListBookedAppointmentsForNoticeParams) ([]db.ListBookedAppointmentsForNoticeRow, error)
}

// QueueRepository は待機列と待ち時間算出用の集計を扱う。
// 状態遷移メソッドは不正遷移のとき pgx.ErrNoRows を返す。
type QueueRepository interface {
	NextTicketNo(ctx context.Context, arg db.NextTicketNoParams) (int32, error)
	EnqueueEntry(ctx context.Context, arg db.EnqueueEntryParams) (db.QueueEntry, error)
	GetQueueEntry(ctx context.Context, id uuid.UUID) (db.QueueEntry, error)
	CallQueueEntry(ctx context.Context, id uuid.UUID) (db.QueueEntry, error)
	StartQueueEntry(ctx context.Context, id uuid.UUID) (db.QueueEntry, error)
	CompleteQueueEntry(ctx context.Context, id uuid.UUID) (db.QueueEntry, error)
	ListQueueEntriesNeedingNearNotice(ctx context.Context, arg db.ListQueueEntriesNeedingNearNoticeParams) ([]db.ListQueueEntriesNeedingNearNoticeRow, error)
	CountWalkInsForDate(ctx context.Context, arg db.CountWalkInsForDateParams) (int64, error)
	CountActiveQueueEntriesForPatient(ctx context.Context, arg db.CountActiveQueueEntriesForPatientParams) (int64, error)
	CancelQueueEntry(ctx context.Context, id uuid.UUID) (db.QueueEntry, error)
	CountAhead(ctx context.Context, arg db.CountAheadParams) (int64, error)
	CountWaiting(ctx context.Context, arg db.CountWaitingParams) (int64, error)
	RecentConsultSeconds(ctx context.Context, arg db.RecentConsultSecondsParams) ([]float64, error)
	CountOnDutyDoctors(ctx context.Context, arg db.CountOnDutyDoctorsParams) (int64, error)
	// CountScheduledDoctors は休診を無視した診療時間内の医師数、CountDoctorsStartingLater は今日この後に診療が始まる医師数。
	CountScheduledDoctors(ctx context.Context, arg db.CountScheduledDoctorsParams) (int64, error)
	CountDoctorsStartingLater(ctx context.Context, arg db.CountDoctorsStartingLaterParams) (int64, error)
	// ListStaffQueue は氏名・症状メモを含むためスタッフ専用。公開 API では使わない。
	ListStaffQueue(ctx context.Context, arg db.ListStaffQueueParams) ([]db.ListStaffQueueRow, error)
	GetQueueEntryForPatient(ctx context.Context, arg db.GetQueueEntryForPatientParams) (db.QueueEntry, error)
}

// NotificationRepository は通知アウトボックスを扱う。
type NotificationRepository interface {
	EnqueueNotification(ctx context.Context, arg db.EnqueueNotificationParams) (db.Notification, error)
	ClaimNotifications(ctx context.Context, arg db.ClaimNotificationsParams) ([]db.Notification, error)
	MarkNotificationSent(ctx context.Context, id uuid.UUID) error
	MarkNotificationFailed(ctx context.Context, id uuid.UUID) error
	// EnqueueNotificationOnce は冪等な登録（リマインド/順番接近）。既に登録済みなら 0 を返す。
	EnqueueNotificationOnce(ctx context.Context, arg db.EnqueueNotificationOnceParams) (int64, error)
	RescheduleNotification(ctx context.Context, arg db.RescheduleNotificationParams) error
}

// AdminRepository はマスタ・設定・休診・監査ログを扱う。
type AdminRepository interface {
	ListDepartments(ctx context.Context) ([]db.Department, error)
	// GetDepartment / GetDoctor は存在しないとき pgx.ErrNoRows。
	GetDepartment(ctx context.Context, id int64) (db.Department, error)
	// LockDepartment は Tx 内で診療科の行をロックする（受付の直列化）。
	LockDepartment(ctx context.Context, id int64) (int64, error)
	GetDoctor(ctx context.Context, id int64) (db.Doctor, error)
	ListClosuresFrom(ctx context.Context, after pgtype.Timestamptz) ([]db.ListClosuresFromRow, error)
	ListDoctors(ctx context.Context, departmentID int64) ([]db.Doctor, error)
	GetClinicSettings(ctx context.Context) (db.ClinicSetting, error)
	UpdateClinicSettings(ctx context.Context, arg db.UpdateClinicSettingsParams) (db.ClinicSetting, error)
	UpdateDepartmentSettings(ctx context.Context, arg db.UpdateDepartmentSettingsParams) (db.Department, error)
	DeleteDoctorSchedules(ctx context.Context, doctorID int64) error
	CreateDoctorSchedule(ctx context.Context, arg db.CreateDoctorScheduleParams) error
	ListDoctorSchedules(ctx context.Context, doctorID int64) ([]db.DoctorSchedule, error)
	ListSchedulesByWeekday(ctx context.Context, weekday int16) ([]db.ListSchedulesByWeekdayRow, error)
	CreateClosure(ctx context.Context, arg db.CreateClosureParams) error
	InsertAuditLog(ctx context.Context, arg db.InsertAuditLogParams) error
}

// Repositories は 1 つの接続（または Tx）に束ねた全 repository。
type Repositories struct {
	Slots         SlotRepository
	Appointments  AppointmentRepository
	Queue         QueueRepository
	Notifications NotificationRepository
	Admin         AdminRepository
}

// TxRunner は fn を 1 つのトランザクションで実行する。fn が error を返す・panic した場合は rollback する。
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(Repositories) error) error
}

// *db.Queries がすべての interface を満たすことをコンパイル時に保証する。
var (
	_ SlotRepository         = (*db.Queries)(nil)
	_ AppointmentRepository  = (*db.Queries)(nil)
	_ QueueRepository        = (*db.Queries)(nil)
	_ NotificationRepository = (*db.Queries)(nil)
	_ AdminRepository        = (*db.Queries)(nil)
)

func newRepositories(q *db.Queries) Repositories {
	return Repositories{Slots: q, Appointments: q, Queue: q, Notifications: q, Admin: q}
}
