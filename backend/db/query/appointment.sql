-- IMP 5-clinic-reservation-sqlc-repository TASK-0001 / 予約（REQ-002, REQ-003）
-- 患者向けクエリは patient_id で必ず絞る（IDOR 対策）。

-- name: CreateAppointment :one
INSERT INTO appointments (appointment_no, patient_id, slot_id, visit_type, symptom_note)
VALUES (sqlc.arg(appointment_no), sqlc.arg(patient_id), sqlc.arg(slot_id), sqlc.arg(visit_type), sqlc.arg(symptom_note))
RETURNING *;

-- name: GetAppointment :one
SELECT * FROM appointments WHERE id = sqlc.arg(id) AND patient_id = sqlc.arg(patient_id);

-- name: ListAppointmentsByPatient :many
SELECT a.id, a.appointment_no, a.visit_type, a.symptom_note, a.status, a.canceled_at,
       s.id AS slot_id, s.start_at,
       d.name AS department_name, dr.name AS doctor_name
FROM appointments a
JOIN slots s        ON s.id = a.slot_id
JOIN departments d  ON d.id = s.department_id
JOIN doctors dr     ON dr.id = s.doctor_id
WHERE a.patient_id = $1
ORDER BY s.start_at, a.created_at;

-- name: CancelAppointment :one
UPDATE appointments
SET status = 'canceled', canceled_at = now()
WHERE id = sqlc.arg(id) AND patient_id = sqlc.arg(patient_id) AND status = 'booked'
RETURNING *;

-- name: RescheduleAppointment :one
UPDATE appointments
SET slot_id = sqlc.arg(slot_id)
WHERE id = sqlc.arg(id) AND patient_id = sqlc.arg(patient_id) AND status = 'booked'
RETURNING *;

-- name: CountPatientBookingsAt :one
-- 同一時刻に有効な予約を持つか（REQ-002-AC-5）
SELECT count(*) FROM appointments a
JOIN slots s ON s.id = a.slot_id
WHERE a.patient_id = sqlc.arg(patient_id) AND a.status = 'booked' AND s.start_at = sqlc.arg(start_at);

-- name: ListBookedAppointmentsForNotice :many
-- 休診の影響を受ける予約者（REQ-005-AC-3）。doctor_id NULL なら全医師。
SELECT a.id, a.appointment_no, a.patient_id, s.start_at
FROM appointments a
JOIN slots s ON s.id = a.slot_id
WHERE a.status = 'booked'
  AND (sqlc.narg(doctor_id)::bigint IS NULL OR s.doctor_id = sqlc.narg(doctor_id))
  AND s.start_at >= sqlc.arg(from_at)
  AND s.start_at <  sqlc.arg(to_at)
ORDER BY s.start_at;
