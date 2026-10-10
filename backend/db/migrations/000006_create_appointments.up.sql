-- IMP 4-clinic-reservation-db-migration TASK-0001 / REQ-002, REQ-003
-- symptom_note は要配慮個人情報。このカラムは appointments のみに置く（REQ-007-AC-2）
CREATE TABLE appointments (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_no varchar(32) NOT NULL,
    patient_id     uuid        NOT NULL REFERENCES users (id),
    slot_id        uuid        NOT NULL REFERENCES slots (id),
    visit_type     varchar(10) NOT NULL,
    symptom_note   text        NOT NULL DEFAULT '',
    status         varchar(10) NOT NULL DEFAULT 'booked',
    canceled_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT appointments_no_key UNIQUE (appointment_no),
    CONSTRAINT appointments_visit_type_check CHECK (visit_type IN ('first', 'revisit')),
    CONSTRAINT appointments_status_check CHECK (status IN ('booked', 'canceled', 'completed', 'no_show'))
);
CREATE UNIQUE INDEX appointments_active_patient_slot_key ON appointments (patient_id, slot_id) WHERE status = 'booked';
CREATE INDEX appointments_patient_idx ON appointments (patient_id, status);
CREATE INDEX appointments_slot_idx ON appointments (slot_id);
