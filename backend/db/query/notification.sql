-- IMP 5-clinic-reservation-sqlc-repository TASK-0001 / 通知アウトボックス（REQ-002-AC-3）

-- name: EnqueueNotification :one
INSERT INTO notifications (user_id, kind, payload)
VALUES (sqlc.arg(user_id), sqlc.arg(kind), sqlc.arg(payload))
RETURNING *;

-- name: ClaimNotifications :many
-- 送信対象を取得し、リース（send_after を先送り）して attempts を加算する。
-- FOR UPDATE SKIP LOCKED により複数ワーカーでも重複取得しない。長い Tx を持たない。
UPDATE notifications
SET attempts = attempts + 1,
    send_after = now() + make_interval(secs => sqlc.arg(lease_seconds)::float8)
WHERE id IN (
    SELECT n.id FROM notifications n
    WHERE n.status = 'pending' AND n.send_after <= now()
    ORDER BY n.send_after
    LIMIT sqlc.arg('limit')
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkNotificationSent :exec
UPDATE notifications SET status = 'sent' WHERE id = $1;

-- name: MarkNotificationFailed :exec
UPDATE notifications SET status = 'failed' WHERE id = $1;
