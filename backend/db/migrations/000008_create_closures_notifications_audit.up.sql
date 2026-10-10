-- IMP 4-clinic-reservation-db-migration TASK-0001 / REQ-005-AC-3, REQ-007-AC-3
-- doctor_id NULL = 医療機関全体の休診
CREATE TABLE closures (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    doctor_id  bigint      REFERENCES doctors (id),
    period     tstzrange   NOT NULL,
    reason     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT closures_period_check CHECK (NOT isempty(period))
);
CREATE INDEX closures_period_idx ON closures USING gist (period);

-- 通知アウトボックス（ワーカーが FOR UPDATE SKIP LOCKED で取得する）
CREATE TABLE notifications (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       varchar(40) NOT NULL,
    payload    jsonb       NOT NULL DEFAULT '{}',
    status     varchar(10) NOT NULL DEFAULT 'pending',
    attempts   integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    send_after timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notifications_status_check CHECK (status IN ('pending', 'sent', 'failed'))
);
CREATE INDEX notifications_status_send_after_idx ON notifications (status, send_after);

-- 監査ログ。ユーザー削除後も記録は残す（actor_id を NULL 化）
CREATE TABLE audit_logs (
    id       bigint       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id uuid         REFERENCES users (id) ON DELETE SET NULL,
    action   varchar(60)  NOT NULL,
    target   varchar(120) NOT NULL DEFAULT '',
    at       timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, at);
CREATE INDEX audit_logs_at_idx ON audit_logs (at);
