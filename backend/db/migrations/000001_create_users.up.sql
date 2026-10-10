-- IMP 1-user-auth-db-migration TASK-0001 / REQ-001-AC-1
CREATE TABLE users (
    id             uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    email          varchar(255) NOT NULL,
    password_hash  varchar(72)  NOT NULL,
    name           varchar(100) NOT NULL,
    email_verified boolean      NOT NULL DEFAULT false,
    role           varchar(20)  NOT NULL DEFAULT 'patient',
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT users_role_check CHECK (role IN ('patient', 'staff', 'clinic_admin', 'system_admin'))
);

-- email は service 層で小文字正規化してから保存する
CREATE UNIQUE INDEX users_email_key ON users (email);
