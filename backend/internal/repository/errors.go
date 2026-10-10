package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound は対象の行が存在しないことを表す（pgx.ErrNoRows を変換したもの）。
	ErrNotFound = errors.New("repository: not found")
	// ErrEmailTaken は email の一意制約違反を表す。
	ErrEmailTaken = errors.New("repository: email already taken")
)

// マイグレーション上の一意制約（インデックス）名。どの制約に違反したかで業務上の意味が変わる。
const (
	ConstraintAppointmentNo     = "appointments_no_key"                  // 予約番号の衝突 → 番号を変えて再試行
	ConstraintActivePatientSlot = "appointments_active_patient_slot_key" // 同一患者の同一枠の二重予約
	ConstraintQueueTicket       = "queue_entries_ticket_key"             // 受付番号の衝突 → 採番し直して再試行
	ConstraintQueueAppointment  = "queue_entries_appointment_key"        // 同じ予約の二重の来院受付
)

// UniqueViolationConstraint は一意制約違反(23505)のとき、違反した制約名を返す。
func UniqueViolationConstraint(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}
