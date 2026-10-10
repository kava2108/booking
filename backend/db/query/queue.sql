-- IMP 5-clinic-reservation-sqlc-repository TASK-0001 / 待機列（REQ-004, 005, 008）
-- 状態遷移は「現在の状態」を WHERE に含める。0 行（pgx.ErrNoRows）= 不正遷移（service が 409 にする）。

-- name: NextTicketNo :one
-- 採番は MAX+1。同時採番の衝突は UNIQUE 違反（23505）になるため service が再試行する。
SELECT (COALESCE(MAX(ticket_no), 0) + 1)::int AS ticket_no
FROM queue_entries
WHERE department_id = sqlc.arg(department_id) AND service_date = sqlc.arg(service_date);

-- name: EnqueueEntry :one
INSERT INTO queue_entries (department_id, appointment_id, patient_id, display_name, source, service_date, ticket_no, sort_time, checked_in_at)
VALUES (sqlc.arg(department_id), sqlc.arg(appointment_id), sqlc.arg(patient_id), sqlc.arg(display_name),
        sqlc.arg(source), sqlc.arg(service_date), sqlc.arg(ticket_no), sqlc.arg(sort_time), sqlc.arg(checked_in_at))
RETURNING *;

-- name: GetQueueEntry :one
SELECT * FROM queue_entries WHERE id = $1;

-- name: CallQueueEntry :one
UPDATE queue_entries SET status = 'called'
WHERE id = $1 AND status = 'waiting'
RETURNING *;

-- name: StartQueueEntry :one
UPDATE queue_entries SET status = 'in_consultation', started_at = now()
WHERE id = $1 AND status = 'called'
RETURNING *;

-- name: CompleteQueueEntry :one
UPDATE queue_entries SET status = 'done', completed_at = now()
WHERE id = $1 AND status = 'in_consultation'
RETURNING *;

-- name: CancelQueueEntry :one
UPDATE queue_entries SET status = 'canceled'
WHERE id = $1 AND status IN ('waiting', 'called')
RETURNING *;

-- name: CountAhead :one
-- 自分（sort_time）より前にいる人数。診察中も含め、完了・キャンセルは含めない。
SELECT count(*) FROM queue_entries
WHERE department_id = sqlc.arg(department_id) AND service_date = sqlc.arg(service_date)
  AND status IN ('waiting', 'called', 'in_consultation')
  AND sort_time < sqlc.arg(sort_time);

-- name: CountWaiting :one
SELECT count(*) FROM queue_entries
WHERE department_id = sqlc.arg(department_id) AND service_date = sqlc.arg(service_date)
  AND status IN ('waiting', 'called', 'in_consultation');

-- name: RecentConsultSeconds :many
-- 直近の診察所要時間（秒）。完了時刻の新しい順。
SELECT EXTRACT(EPOCH FROM (completed_at - started_at))::float8 AS seconds
FROM queue_entries
WHERE department_id = sqlc.arg(department_id) AND service_date = sqlc.arg(service_date)
  AND status = 'done' AND started_at IS NOT NULL AND completed_at IS NOT NULL
ORDER BY completed_at DESC
LIMIT sqlc.arg('limit');

-- name: CountOnDutyDoctors :one
-- 現在診療時間内（Asia/Tokyo の曜日・時刻）で、休診でないアクティブな医師数。
SELECT count(*) FROM doctors d
WHERE d.department_id = sqlc.arg(department_id) AND d.active
  AND EXISTS (
    SELECT 1 FROM doctor_schedules s
    WHERE s.doctor_id = d.id
      AND s.weekday = EXTRACT(DOW FROM (sqlc.arg(at)::timestamptz AT TIME ZONE 'Asia/Tokyo'))
      AND (sqlc.arg(at)::timestamptz AT TIME ZONE 'Asia/Tokyo')::time >= s.start_time
      AND (sqlc.arg(at)::timestamptz AT TIME ZONE 'Asia/Tokyo')::time <  s.end_time
  )
  AND NOT EXISTS (
    SELECT 1 FROM closures c
    WHERE (c.doctor_id = d.id OR c.doctor_id IS NULL) AND c.period @> sqlc.arg(at)::timestamptz
  );

-- name: ListStaffQueue :many
-- スタッフ専用。氏名・症状メモを含む（公開 API では使わない）。
SELECT q.id, q.ticket_no, q.source, q.status, q.sort_time, q.checked_in_at, q.started_at, q.completed_at,
       COALESCE(u.name, q.display_name, '')::text AS patient_name,
       COALESCE(a.symptom_note, '')::text AS symptom_note
FROM queue_entries q
LEFT JOIN users u        ON u.id = q.patient_id
LEFT JOIN appointments a ON a.id = q.appointment_id
WHERE q.department_id = sqlc.arg(department_id) AND q.service_date = sqlc.arg(service_date)
  AND q.status IN ('waiting', 'called', 'in_consultation')
ORDER BY q.sort_time, q.ticket_no;

-- name: GetQueueEntryForPatient :one
-- 患者本人の順番参照。patient_id で絞る（IDOR 対策）。
SELECT * FROM queue_entries
WHERE appointment_id = sqlc.arg(appointment_id) AND patient_id = sqlc.arg(patient_id);

-- name: CountWalkInsForDate :one
-- 当日受付の定員判定用（キャンセル済みと予約由来は数えない）。
SELECT count(*) FROM queue_entries
WHERE department_id = sqlc.arg(department_id) AND service_date = sqlc.arg(service_date)
  AND source = 'walk_in' AND status <> 'canceled';

-- name: CountActiveQueueEntriesForPatient :one
-- 同じ患者が同じ診療科の列に重複して並ばないための判定。
SELECT count(*) FROM queue_entries
WHERE department_id = sqlc.arg(department_id) AND service_date = sqlc.arg(service_date)
  AND patient_id = sqlc.arg(patient_id) AND status IN ('waiting', 'called', 'in_consultation');

-- name: ListQueueEntriesNeedingNearNotice :many
-- 待機中で、前にいる人数が max_ahead 以下になった患者（アカウントあり）のうち、通知未登録のもの。
SELECT q.id, q.patient_id, q.ticket_no, d.name AS department_name,
       (SELECT count(*) FROM queue_entries p
        WHERE p.department_id = q.department_id AND p.service_date = q.service_date
          AND p.status IN ('waiting', 'called', 'in_consultation') AND p.sort_time < q.sort_time)::bigint AS ahead
FROM queue_entries q
JOIN departments d ON d.id = q.department_id
WHERE q.service_date = sqlc.arg(service_date) AND q.status = 'waiting' AND q.patient_id IS NOT NULL
  AND (SELECT count(*) FROM queue_entries p
       WHERE p.department_id = q.department_id AND p.service_date = q.service_date
         AND p.status IN ('waiting', 'called', 'in_consultation') AND p.sort_time < q.sort_time) <= sqlc.arg(max_ahead)::bigint
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.kind = 'queue_near' AND n.payload ->> 'queue_entry_id' = q.id::text
  )
ORDER BY q.sort_time;
