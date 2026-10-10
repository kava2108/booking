-- IMP 4-clinic-reservation-db-migration TASK-0001 / REQ-005, REQ-008
-- 予約・当日受付を単一の待機列で管理する。
-- sort_time: 予約 = GREATEST(予約時刻, 来院受付時刻) / 当日受付 = 受付時刻（service 層で設定）
-- patient_id NULL = スタッフ代行の無アカウント受付（display_name はスタッフ専用、公開 API に出さない）
CREATE TABLE queue_entries (
    id             uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    department_id  bigint       NOT NULL REFERENCES departments (id),
    appointment_id uuid         REFERENCES appointments (id),
    patient_id     uuid         REFERENCES users (id),
    display_name   varchar(100),
    source         varchar(12)  NOT NULL,
    service_date   date         NOT NULL,
    ticket_no      integer      NOT NULL CHECK (ticket_no > 0),
    sort_time      timestamptz  NOT NULL,
    status         varchar(16)  NOT NULL DEFAULT 'waiting',
    checked_in_at  timestamptz  NOT NULL,
    started_at     timestamptz,
    completed_at   timestamptz,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT queue_entries_source_check CHECK (source IN ('reservation', 'walk_in')),
    CONSTRAINT queue_entries_status_check CHECK (status IN ('waiting', 'called', 'in_consultation', 'done', 'canceled')),
    CONSTRAINT queue_entries_source_appointment_check CHECK (
        (source = 'reservation' AND appointment_id IS NOT NULL) OR
        (source = 'walk_in'     AND appointment_id IS NULL)
    ),
    CONSTRAINT queue_entries_identity_check CHECK (patient_id IS NOT NULL OR display_name IS NOT NULL),
    CONSTRAINT queue_entries_ticket_key UNIQUE (department_id, service_date, ticket_no)
);
CREATE UNIQUE INDEX queue_entries_appointment_key ON queue_entries (appointment_id) WHERE appointment_id IS NOT NULL;
CREATE INDEX queue_entries_dept_status_sort_idx ON queue_entries (department_id, status, sort_time);
CREATE INDEX queue_entries_completed_idx ON queue_entries (department_id, completed_at) WHERE status = 'done';
