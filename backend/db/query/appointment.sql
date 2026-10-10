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
-- 通知の作成に必要な枠 ID・診療科名・医師名を含む。
SELECT a.id, a.appointment_no, a.patient_id, s.id AS slot_id, s.start_at, d.name AS department_name, dr.name AS doctor_name
FROM appointments a
JOIN slots s       ON s.id = a.slot_id
JOIN departments d ON d.id = s.department_id
JOIN doctors dr    ON dr.id = s.doctor_id
WHERE a.status = 'booked'
  AND (sqlc.narg(doctor_id)::bigint IS NULL OR s.doctor_id = sqlc.narg(doctor_id))
  AND s.start_at >= sqlc.arg(from_at)
  AND s.start_at <  sqlc.arg(to_at)
ORDER BY s.start_at;

-- name: GetAppointmentWithSlot :one
-- スタッフの来院受付用。患者スコープなしなので StaffOnly のハンドラーからのみ使うこと。
SELECT a.id, a.appointment_no, a.patient_id, a.status, s.id AS slot_id, s.department_id, s.start_at
FROM appointments a
JOIN slots s ON s.id = a.slot_id
WHERE a.id = $1;

-- name: CompleteAppointment :execrows
-- 診察完了時に予約を completed にする。更新件数 0 = 既に完了/取消済み。
UPDATE appointments SET status = 'completed' WHERE id = $1 AND status = 'booked';

-- name: ListAppointmentsNeedingReminder :many
-- 診療日が [from_at, to_at) の有効な予約のうち、リマインド未登録のもの。
SELECT a.id, a.appointment_no, a.patient_id, s.start_at, d.name AS department_name, dr.name AS doctor_name
FROM appointments a
JOIN slots s       ON s.id = a.slot_id
JOIN departments d ON d.id = s.department_id
JOIN doctors dr    ON dr.id = s.doctor_id
WHERE a.status = 'booked'
  AND s.start_at >= sqlc.arg(from_at) AND s.start_at < sqlc.arg(to_at)
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.kind = 'appointment_reminder' AND n.payload ->> 'appointment_id' = a.id::text
  )
ORDER BY s.start_at;

-- name: ListAppointmentsForStaff :many
-- スタッフの来院受付用: 指定診療科の [from_at, to_at) の予約（有効・完了のみ）。氏名を含むため StaffOnly + 監査で保護する。
SELECT a.id, a.appointment_no, a.status, a.visit_type, s.start_at,
       dr.name AS doctor_name, u.name AS patient_name,
       EXISTS (SELECT 1 FROM queue_entries q WHERE q.appointment_id = a.id) AS checked_in
FROM appointments a
JOIN slots s    ON s.id = a.slot_id
JOIN doctors dr ON dr.id = s.doctor_id
JOIN users u    ON u.id = a.patient_id
WHERE s.department_id = sqlc.arg(department_id)
  AND s.start_at >= sqlc.arg(from_at) AND s.start_at < sqlc.arg(to_at)
  AND a.status IN ('booked', 'completed')
ORDER BY s.start_at, a.created_at;
