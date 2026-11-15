-- Mawarid Sprint 1 — initial schema
-- Source of truth: approved ERD "ERD 2" + Sprint 1 additions (see docs/DB_SCHEMA.md)
-- All timestamps are UTC timestamptz.

BEGIN;

-- ---------------------------------------------------------------------------
-- Roles & permissions (ERD: Role, Permission, many-to-many)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(40) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS permissions (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(80) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ---------------------------------------------------------------------------
-- Users (ERD: User)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id                  BIGSERIAL PRIMARY KEY,
    role_id             BIGINT NOT NULL REFERENCES roles(id),
    full_name           VARCHAR(120) NOT NULL,
    phone_number        VARCHAR(20) NOT NULL UNIQUE,
    password_hash       VARCHAR(256) NOT NULL,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    id_number           VARCHAR(40),
    id_image_url        VARCHAR(512),
    user_type           VARCHAR(40),
    -- Sprint 1 additions
    status              VARCHAR(24) NOT NULL DEFAULT 'pending_approval'
                        CHECK (status IN ('pending_approval','active','rejected','suspended')),
    phone_verified_at   TIMESTAMPTZ,
    verification_status VARCHAR(24) NOT NULL DEFAULT 'unverified'
                        CHECK (verification_status IN ('unverified','pending_review','verified','rejected')),
    admin_note          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);

-- ---------------------------------------------------------------------------
-- Profiles (ERD: Customer, Home, Businiss, Provider, Driver, Address)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS customers (
    user_id    BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(20) NOT NULL CHECK (type IN ('home','business')),
    subtype    VARCHAR(30) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS homes (
    user_id       BIGINT PRIMARY KEY REFERENCES customers(user_id) ON DELETE CASCADE,
    family_count  INT CHECK (family_count >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ERD column name "cmmercial_image_url" kept for compatibility.
CREATE TABLE IF NOT EXISTS businesses (
    user_id                    BIGINT PRIMARY KEY REFERENCES customers(user_id) ON DELETE CASCADE,
    commercial_registry_info   VARCHAR(255),           -- SRS FR-CON-03
    cmmercial_image_url        VARCHAR(512),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS providers (
    user_id             BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    provider_name       VARCHAR(120) NOT NULL,
    provider_description VARCHAR(500),               -- SRS FR-PRV-01
    start_time          VARCHAR(8),                  -- SRS NFR-DAT-01 "HH:MM"
    end_time            VARCHAR(8),
    logo_url            VARCHAR(512),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS drivers (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    car_number   VARCHAR(40),                          -- SRS FR-CON-03 plate number
    vehicle_type VARCHAR(60),                          -- SRS FR-CON-03 vehicle type
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS addresses (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    city               VARCHAR(80) NOT NULL,
    district           VARCHAR(80),
    street             VARCHAR(255),
    additional_info    VARCHAR(255),
    address_image_url  VARCHAR(512),
    latitude           DOUBLE PRECISION,
    longitude          DOUBLE PRECISION,
    is_default         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_addresses_user_id ON addresses(user_id);
CREATE INDEX IF NOT EXISTS idx_addresses_deleted_at ON addresses(deleted_at);

CREATE TABLE IF NOT EXISTS notifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      VARCHAR(200) NOT NULL,
    content    TEXT NOT NULL,
    is_read    BOOLEAN NOT NULL DEFAULT FALSE,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);

-- ---------------------------------------------------------------------------
-- Sprint 1 additions — auth & security infrastructure
-- ---------------------------------------------------------------------------

-- OTP codes: only the hash is stored; one live code per (phone, purpose).
CREATE TABLE IF NOT EXISTS otp_codes (
    id           BIGSERIAL PRIMARY KEY,
    phone_number VARCHAR(20) NOT NULL,
    purpose      VARCHAR(24) NOT NULL CHECK (purpose IN ('register','login','password_reset')),
    code_hash    VARCHAR(128) NOT NULL,
    attempts     INT NOT NULL DEFAULT 0,
    consumed_at  TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT idx_otp_scope UNIQUE (phone_number, purpose)
);
CREATE INDEX IF NOT EXISTS idx_otp_codes_expires_at ON otp_codes(expires_at);

-- Refresh sessions (opaque refresh token stored as SHA-256 hash only).
CREATE TABLE IF NOT EXISTS auth_sessions (
    id                 VARCHAR(64) PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,
    user_agent         VARCHAR(255),
    ip                 VARCHAR(45),
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    last_used_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);

-- Persistent login-attempt audit (combined with the Redis limiter).
CREATE TABLE IF NOT EXISTS login_attempts (
    id             BIGSERIAL PRIMARY KEY,
    phone          VARCHAR(20) NOT NULL,
    ip             VARCHAR(45),
    success        BOOLEAN NOT NULL DEFAULT FALSE,
    failure_reason VARCHAR(60),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_scope ON login_attempts(phone, created_at);

-- Uploaded identity/commercial documents (private storage).
CREATE TABLE IF NOT EXISTS identity_documents (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_type VARCHAR(40) NOT NULL CHECK (document_type IN ('national_id','commercial_license','other')),
    file_path     VARCHAR(512) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    content_type  VARCHAR(100) NOT NULL CHECK (content_type IN ('image/jpeg','image/png','application/pdf')),
    size_bytes    BIGINT NOT NULL CHECK (size_bytes > 0),
    sha256        VARCHAR(64) NOT NULL,
    status        VARCHAR(24) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    reviewer_id   BIGINT REFERENCES users(id),
    review_note   TEXT,
    reviewed_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_identity_documents_user_id ON identity_documents(user_id);
CREATE INDEX IF NOT EXISTS idx_identity_documents_status ON identity_documents(status);
CREATE INDEX IF NOT EXISTS idx_identity_documents_sha256 ON identity_documents(sha256);

-- Privileged-action audit log (no sensitive identity data).
CREATE TABLE IF NOT EXISTS audit_logs (
    id             BIGSERIAL PRIMARY KEY,
    actor_user_id  BIGINT REFERENCES users(id),
    target_user_id BIGINT REFERENCES users(id),
    action         VARCHAR(80) NOT NULL,
    details        TEXT,
    ip             VARCHAR(45),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

-- ---------------------------------------------------------------------------
-- Sprint 1 additions — provider locations & working hours
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS provider_locations (
    id              BIGSERIAL PRIMARY KEY,
    provider_id     BIGINT NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL,
    city            VARCHAR(80) NOT NULL,
    district        VARCHAR(80),
    street          VARCHAR(255),
    additional_info VARCHAR(255),
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_provider_locations_provider_id ON provider_locations(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_locations_city ON provider_locations(city);  -- NFR-DAT-01
CREATE INDEX IF NOT EXISTS idx_provider_locations_deleted_at ON provider_locations(deleted_at);

CREATE TABLE IF NOT EXISTS provider_working_hours (
    id         BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    location_id BIGINT REFERENCES provider_locations(id) ON DELETE SET NULL,
    day_of_week INT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6), -- 0=Sunday
    start_time  VARCHAR(8) NOT NULL,
    end_time    VARCHAR(8) NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (start_time <> end_time)
);
CREATE INDEX IF NOT EXISTS idx_provider_working_hours_provider_id ON provider_working_hours(provider_id);

-- ---------------------------------------------------------------------------
-- Roles seed
-- ---------------------------------------------------------------------------
INSERT INTO roles (name) VALUES
    ('consumer'),
    ('provider'),
    ('driver'),
    ('admin')
ON CONFLICT (name) DO NOTHING;

INSERT INTO permissions (name) VALUES
    ('orders:create'),
    ('orders:read:own'),
    ('provider:manage:own'),
    ('admin:accounts:review'),
    ('admin:audit:read')
ON CONFLICT (name) DO NOTHING;

COMMIT;
