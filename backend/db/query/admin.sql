-- IMP 5-clinic-reservation-sqlc-repository TASK-0001 / マスタ・設定・休診・監査（REQ-006, REQ-005-AC-3, REQ-007-AC-3）

-- name: ListDepartments :many
SELECT * FROM departments ORDER BY id;

-- name: ListDoctors :many
SELECT * FROM doctors WHERE department_id = $1 AND active ORDER BY id;

-- name: GetClinicSettings :one
SELECT * FROM clinic_settings WHERE id = 1;

-- name: UpdateClinicSettings :one
UPDATE clinic_settings
SET cancel_deadline_minutes = sqlc.arg(cancel_deadline_minutes),
    booking_window_days = sqlc.arg(booking_window_days),
    updated_at = now()
WHERE id = 1
RETURNING *;

-- name: UpdateDepartmentSettings :one
UPDATE departments
SET standard_consult_minutes = sqlc.arg(standard_consult_minutes),
    walkin_cutoff = sqlc.arg(walkin_cutoff),
    walkin_capacity = sqlc.arg(walkin_capacity)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteDoctorSchedules :exec
DELETE FROM doctor_schedules WHERE doctor_id = $1;

-- name: CreateDoctorSchedule :exec
INSERT INTO doctor_schedules (doctor_id, weekday, start_time, end_time, slot_minutes, capacity)
VALUES (sqlc.arg(doctor_id), sqlc.arg(weekday), sqlc.arg(start_time), sqlc.arg(end_time), sqlc.arg(slot_minutes), sqlc.arg(capacity));

-- name: ListSchedulesByWeekday :many
-- 枠生成用。診療科 ID を併せて返す。非アクティブな医師は除く。
SELECT s.id, s.doctor_id, d.department_id, s.weekday, s.start_time, s.end_time, s.slot_minutes, s.capacity
FROM doctor_schedules s
JOIN doctors d ON d.id = s.doctor_id
WHERE s.weekday = $1 AND d.active
ORDER BY s.doctor_id, s.start_time;

-- name: CreateClosure :exec
-- doctor_id NULL = 医療機関全体の休診
INSERT INTO closures (doctor_id, period, reason)
VALUES (sqlc.narg(doctor_id), tstzrange(sqlc.arg(from_at)::timestamptz, sqlc.arg(to_at)::timestamptz), sqlc.arg(reason));

-- name: InsertAuditLog :exec
-- 対象 ID とアクションのみ記録する。症状メモ等の本文は記録しない。
INSERT INTO audit_logs (actor_id, action, target)
VALUES (sqlc.arg(actor_id), sqlc.arg(action), sqlc.arg(target));

-- name: GetDepartment :one
SELECT * FROM departments WHERE id = $1;

-- name: GetDoctor :one
SELECT * FROM doctors WHERE id = $1;

-- name: ListClosuresFrom :many
-- 終了が基準時刻より後の休診。枠生成時に既存の休診期間を再適用するために使う。
SELECT id, doctor_id, lower(period)::timestamptz AS from_at, upper(period)::timestamptz AS to_at, reason
FROM closures
WHERE upper(period) > sqlc.arg(after)::timestamptz
ORDER BY lower(period);

-- name: LockDepartment :one
-- 診療科の行をロックする（トランザクション内）。同じ診療科への同時の受付を直列化し、
-- 当日受付の定員判定と受付番号の採番の競合を防ぐ。
SELECT id FROM departments WHERE id = $1 FOR UPDATE;
