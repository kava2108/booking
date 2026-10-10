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

-- name: EnqueueNotificationOnce :execrows
-- リマインド/順番接近通知用の冪等な登録。部分ユニークインデックス(000009)により、同じ予約・同じ待機列項目では
-- 並行しても 1 件だけ登録される。影響行数 0 = 既に登録済み。
INSERT INTO notifications (user_id, kind, payload)
VALUES (sqlc.arg(user_id), sqlc.arg(kind), sqlc.arg(payload))
ON CONFLICT DO NOTHING;

-- name: RescheduleNotification :exec
-- 送信失敗時のバックオフ。pending のものだけ send_after を後ろ倒しする。
UPDATE notifications
SET send_after = now() + make_interval(secs => sqlc.arg(delay_seconds)::float8)
WHERE id = sqlc.arg(id) AND status = 'pending';
