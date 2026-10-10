-- IMP 5-clinic-reservation-sqlc-repository TASK-0001 / 枠（REQ-001, REQ-002-AC-2）

-- name: ListSlots :many
-- 満席・休診も含めて返す。選択可否の判定は service が行う（REQ-001-AC-3）。
SELECT * FROM slots
WHERE department_id = sqlc.arg(department_id)
  AND (sqlc.narg(doctor_id)::bigint IS NULL OR doctor_id = sqlc.narg(doctor_id))
  AND start_at >= sqlc.arg(from_at)
  AND start_at <  sqlc.arg(to_at)
ORDER BY start_at, doctor_id;

-- name: GetSlot :one
SELECT * FROM slots WHERE id = $1;

-- name: CreateSlotIfAbsent :exec
-- 既存の枠は上書きしない（日次生成ジョブの再実行を安全にする）
INSERT INTO slots (department_id, doctor_id, start_at, capacity)
VALUES (sqlc.arg(department_id), sqlc.arg(doctor_id), sqlc.arg(start_at), sqlc.arg(capacity))
ON CONFLICT (doctor_id, start_at) DO NOTHING;

-- name: ReserveSlot :one
-- 排他確保。単一の条件付き UPDATE で定員超過を防ぐ。0 行（pgx.ErrNoRows）= 満席または休診。
UPDATE slots
SET booked_count = booked_count + 1
WHERE id = $1 AND status = 'open' AND booked_count < capacity
RETURNING *;

-- name: ReleaseSlot :exec
UPDATE slots
SET booked_count = booked_count - 1
WHERE id = $1 AND booked_count > 0;

-- name: CloseSlots :many
-- doctor_id が NULL なら全医師。休診設定で使う（REQ-005-AC-3）。
UPDATE slots
SET status = 'closed'
WHERE (sqlc.narg(doctor_id)::bigint IS NULL OR doctor_id = sqlc.narg(doctor_id))
  AND start_at >= sqlc.arg(from_at)
  AND start_at <  sqlc.arg(to_at)
  AND status = 'open'
RETURNING id;
