-- IMP 4-clinic-reservation-db-migration TASK-0001 / REQ-002-AC-2
-- booked_count の更新は「WHERE booked_count < capacity」付きの単一 UPDATE で行う。CHECK は最終防壁。
CREATE TABLE slots (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    department_id bigint      NOT NULL REFERENCES departments (id),
    doctor_id     bigint      NOT NULL REFERENCES doctors (id),
    start_at      timestamptz NOT NULL,
    capacity      integer     NOT NULL CHECK (capacity > 0),
    booked_count  integer     NOT NULL DEFAULT 0 CHECK (booked_count >= 0),
    status        varchar(10) NOT NULL DEFAULT 'open',
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT slots_status_check CHECK (status IN ('open', 'closed')),
    CONSTRAINT slots_booked_within_capacity CHECK (booked_count <= capacity),
    CONSTRAINT slots_doctor_start_key UNIQUE (doctor_id, start_at)
);
CREATE INDEX slots_department_start_idx ON slots (department_id, start_at);
