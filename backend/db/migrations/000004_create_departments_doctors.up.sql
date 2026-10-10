-- IMP 4-clinic-reservation-db-migration TASK-0001 / REQ-006-AC-1, REQ-006-AC-2
CREATE TABLE clinic_settings (
    id                      smallint    PRIMARY KEY DEFAULT 1,
    cancel_deadline_minutes integer     NOT NULL DEFAULT 60  CHECK (cancel_deadline_minutes >= 0),
    booking_window_days     integer     NOT NULL DEFAULT 30  CHECK (booking_window_days > 0),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT clinic_settings_singleton CHECK (id = 1)
);
INSERT INTO clinic_settings (id) VALUES (1);

CREATE TABLE departments (
    id                       bigint       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                     varchar(100) NOT NULL,
    standard_consult_minutes integer      CHECK (standard_consult_minutes > 0),
    walkin_cutoff            time,
    walkin_capacity          integer      CHECK (walkin_capacity > 0),
    created_at               timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT departments_name_key UNIQUE (name)
);

CREATE TABLE doctors (
    id            bigint       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    department_id bigint       NOT NULL REFERENCES departments (id),
    name          varchar(100) NOT NULL,
    active        boolean      NOT NULL DEFAULT true,
    created_at    timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX doctors_department_id_idx ON doctors (department_id);

CREATE TABLE doctor_schedules (
    id           bigint   GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    doctor_id    bigint   NOT NULL REFERENCES doctors (id) ON DELETE CASCADE,
    weekday      smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_time   time     NOT NULL,
    end_time     time     NOT NULL,
    slot_minutes integer  NOT NULL CHECK (slot_minutes > 0),
    capacity     integer  NOT NULL CHECK (capacity > 0),
    CONSTRAINT doctor_schedules_time_check CHECK (end_time > start_time)
);
CREATE INDEX doctor_schedules_doctor_id_idx ON doctor_schedules (doctor_id, weekday);
