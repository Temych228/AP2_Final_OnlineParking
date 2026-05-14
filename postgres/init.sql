-- ─── Создание баз данных ──────────────────────────────────────────────────────

CREATE DATABASE user_service;
CREATE DATABASE auth_service;
CREATE DATABASE notification_service;
CREATE DATABASE booking_service;
CREATE DATABASE parking_service;
CREATE DATABASE payment_service;

-- ─── Права доступа ───────────────────────────────────────────────────────────
GRANT ALL PRIVILEGES ON DATABASE user_service       TO parking;
GRANT ALL PRIVILEGES ON DATABASE auth_service       TO parking;
GRANT ALL PRIVILEGES ON DATABASE notification_service TO parking;
GRANT ALL PRIVILEGES ON DATABASE booking_service    TO parking;
GRANT ALL PRIVILEGES ON DATABASE parking_service    TO parking;
GRANT ALL PRIVILEGES ON DATABASE payment_service    TO parking;

-- ─── user_service schema ─────────────────────────────────────────────────────
\c user_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
                                     id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL,
    first_name  TEXT NOT NULL,
    last_name   TEXT NOT NULL,
    phone       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user',
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    is_banned   BOOLEAN NOT NULL DEFAULT FALSE,
    ban_reason  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL
    );

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_active_idx
    ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS users_role_idx       ON users (role);
CREATE INDEX IF NOT EXISTS users_deleted_at_idx ON users (deleted_at);

-- ─── auth_service schema ─────────────────────────────────────────────────────
\c auth_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS auth_users (
                                          id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_verified   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE TABLE IF NOT EXISTS refresh_tokens (
                                              id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx   ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx ON refresh_tokens(expires_at);

-- ─── notification_service schema ─────────────────────────────────────────────
\c notification_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS notifications (
                                             id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    TEXT NOT NULL,
    type       TEXT NOT NULL,
    subject    TEXT NOT NULL,
    body       TEXT NOT NULL,
    is_read    BOOLEAN NOT NULL DEFAULT FALSE,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at    TIMESTAMPTZ NULL
    );

CREATE TABLE IF NOT EXISTS notification_preferences (
                                                        user_id         TEXT PRIMARY KEY,
                                                        email_enabled   BOOLEAN NOT NULL DEFAULT TRUE,
                                                        sms_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
                                                        push_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
                                                        marketing_emails BOOLEAN NOT NULL DEFAULT FALSE,
                                                        updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS notifications_user_id_idx    ON notifications(user_id);
CREATE INDEX IF NOT EXISTS notifications_is_read_idx    ON notifications(is_read);
CREATE INDEX IF NOT EXISTS notifications_created_at_idx ON notifications(created_at DESC);

-- ─── booking_service schema ───────────────────────────────────────────────────
\c booking_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS bookings (
                                        id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    parking_id    BIGINT NOT NULL,
    spot_id       BIGINT NOT NULL,
    vehicle_plate TEXT NOT NULL,
    start_time    TIMESTAMPTZ NOT NULL,
    end_time      TIMESTAMPTZ NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    cancel_reason TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at  TIMESTAMPTZ NULL,
    CHECK (end_time > start_time)
    );

CREATE INDEX IF NOT EXISTS bookings_user_id_idx    ON bookings (user_id);
CREATE INDEX IF NOT EXISTS bookings_parking_id_idx ON bookings (parking_id);
CREATE INDEX IF NOT EXISTS bookings_spot_id_idx    ON bookings (spot_id);
CREATE INDEX IF NOT EXISTS bookings_status_idx     ON bookings (status);
CREATE INDEX IF NOT EXISTS bookings_start_time_idx ON bookings (start_time);
CREATE INDEX IF NOT EXISTS bookings_end_time_idx   ON bookings (end_time);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'bookings_spot_time_overlap_excl'
    ) THEN
ALTER TABLE bookings
    ADD CONSTRAINT bookings_spot_time_overlap_excl
    EXCLUDE USING gist (
                spot_id WITH =,
                tstzrange(start_time, end_time, '[)') WITH &&
            )
            WHERE (status IN ('pending', 'confirmed', 'active'));
END IF;
END $$;

-- ─── parking_service schema ───────────────────────────────────────────────────
\c parking_service

CREATE TABLE IF NOT EXISTS parkings (
                                        id SERIAL PRIMARY KEY,
                                        name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    total_spots INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

CREATE TABLE IF NOT EXISTS spots (
                                     id SERIAL PRIMARY KEY,
                                     parking_id INT NOT NULL REFERENCES parkings(id) ON DELETE CASCADE,
    number VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'AVAILABLE',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

CREATE TABLE IF NOT EXISTS tariffs (
                                       id SERIAL PRIMARY KEY,
                                       parking_id INT NOT NULL REFERENCES parkings(id) ON DELETE CASCADE,
    price_per_hour DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

CREATE INDEX IF NOT EXISTS spots_parking_id_idx ON spots(parking_id);
CREATE INDEX IF NOT EXISTS tariffs_parking_id_idx ON tariffs(parking_id);

-- ─── payment_service schema ───────────────────────────────────────────────────
\c payment_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments (
                                        id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id          UUID NOT NULL,
    user_id             UUID NOT NULL,
    parking_id          BIGINT NOT NULL,
    spot_id             BIGINT NOT NULL,
    amount              NUMERIC(10, 2) NOT NULL,
    method              TEXT NOT NULL DEFAULT 'card',
    status              TEXT NOT NULL DEFAULT 'pending',
    provider_payment_id TEXT,
    failure_reason      TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at             TIMESTAMPTZ NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS payments_booking_id_idx ON payments(booking_id);
CREATE INDEX IF NOT EXISTS payments_user_id_idx    ON payments(user_id);
CREATE INDEX IF NOT EXISTS payments_status_idx     ON payments(status);-- ─── Создание баз данных ──────────────────────────────────────────────────────

CREATE DATABASE user_service;
CREATE DATABASE auth_service;
CREATE DATABASE notification_service;
CREATE DATABASE booking_service;
CREATE DATABASE parking_service;
CREATE DATABASE payment_service;

-- ─── Права доступа ───────────────────────────────────────────────────────────
GRANT ALL PRIVILEGES ON DATABASE user_service       TO parking;
GRANT ALL PRIVILEGES ON DATABASE auth_service       TO parking;
GRANT ALL PRIVILEGES ON DATABASE notification_service TO parking;
GRANT ALL PRIVILEGES ON DATABASE booking_service    TO parking;
GRANT ALL PRIVILEGES ON DATABASE parking_service    TO parking;
GRANT ALL PRIVILEGES ON DATABASE payment_service    TO parking;

-- ─── user_service schema ─────────────────────────────────────────────────────
\c user_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
                                     id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL,
    first_name  TEXT NOT NULL,
    last_name   TEXT NOT NULL,
    phone       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user',
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    is_banned   BOOLEAN NOT NULL DEFAULT FALSE,
    ban_reason  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL
    );

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_active_idx
    ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS users_role_idx       ON users (role);
CREATE INDEX IF NOT EXISTS users_deleted_at_idx ON users (deleted_at);

-- ─── auth_service schema ─────────────────────────────────────────────────────
\c auth_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS auth_users (
                                          id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_verified   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE TABLE IF NOT EXISTS refresh_tokens (
                                              id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx   ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx ON refresh_tokens(expires_at);

-- ─── notification_service schema ─────────────────────────────────────────────
\c notification_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS notifications (
                                             id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    TEXT NOT NULL,
    type       TEXT NOT NULL,
    subject    TEXT NOT NULL,
    body       TEXT NOT NULL,
    is_read    BOOLEAN NOT NULL DEFAULT FALSE,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at    TIMESTAMPTZ NULL
    );

CREATE TABLE IF NOT EXISTS notification_preferences (
                                                        user_id         TEXT PRIMARY KEY,
                                                        email_enabled   BOOLEAN NOT NULL DEFAULT TRUE,
                                                        sms_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
                                                        push_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
                                                        marketing_emails BOOLEAN NOT NULL DEFAULT FALSE,
                                                        updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS notifications_user_id_idx    ON notifications(user_id);
CREATE INDEX IF NOT EXISTS notifications_is_read_idx    ON notifications(is_read);
CREATE INDEX IF NOT EXISTS notifications_created_at_idx ON notifications(created_at DESC);

-- ─── booking_service schema ───────────────────────────────────────────────────
\c booking_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS bookings (
                                        id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    parking_id    BIGINT NOT NULL,
    spot_id       BIGINT NOT NULL,
    vehicle_plate TEXT NOT NULL,
    start_time    TIMESTAMPTZ NOT NULL,
    end_time      TIMESTAMPTZ NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    cancel_reason TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at  TIMESTAMPTZ NULL,
    CHECK (end_time > start_time)
    );

CREATE INDEX IF NOT EXISTS bookings_user_id_idx    ON bookings (user_id);
CREATE INDEX IF NOT EXISTS bookings_parking_id_idx ON bookings (parking_id);
CREATE INDEX IF NOT EXISTS bookings_spot_id_idx    ON bookings (spot_id);
CREATE INDEX IF NOT EXISTS bookings_status_idx     ON bookings (status);
CREATE INDEX IF NOT EXISTS bookings_start_time_idx ON bookings (start_time);
CREATE INDEX IF NOT EXISTS bookings_end_time_idx   ON bookings (end_time);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'bookings_spot_time_overlap_excl'
    ) THEN
ALTER TABLE bookings
    ADD CONSTRAINT bookings_spot_time_overlap_excl
    EXCLUDE USING gist (
                spot_id WITH =,
                tstzrange(start_time, end_time, '[)') WITH &&
            )
            WHERE (status IN ('pending', 'confirmed', 'active'));
END IF;
END $$;

-- ─── parking_service schema ───────────────────────────────────────────────────
\c parking_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS parkings (
                                        id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    address       TEXT NOT NULL,
    latitude      DOUBLE PRECISION NOT NULL DEFAULT 0,
    longitude     DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_spots   INT NOT NULL,
    available_spots INT NOT NULL DEFAULT 0,
    price_per_hour DECIMAL(10, 2) NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE TABLE IF NOT EXISTS spots (
                                     id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parking_id    UUID NOT NULL REFERENCES parkings(id) ON DELETE CASCADE,
    code          VARCHAR(50) NOT NULL,
    level         VARCHAR(50) NOT NULL DEFAULT 'G',
    spot_type     VARCHAR(50) NOT NULL DEFAULT 'standard',
    vehicle_type  VARCHAR(50) NOT NULL DEFAULT 'car',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    is_occupied   BOOLEAN NOT NULL DEFAULT FALSE,
    is_reserved   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE TABLE IF NOT EXISTS tariffs (
                                       id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parking_id    UUID NOT NULL REFERENCES parkings(id) ON DELETE CASCADE,
    name          VARCHAR(100) NOT NULL,
    price_per_hour DECIMAL(10, 2) NOT NULL,
    vehicle_type  VARCHAR(50) NOT NULL DEFAULT 'car',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS spots_parking_id_idx ON spots(parking_id);
CREATE INDEX IF NOT EXISTS spots_is_active_idx  ON spots(is_active);
CREATE INDEX IF NOT EXISTS tariffs_parking_id_idx ON tariffs(parking_id);

-- ─── payment_service schema ───────────────────────────────────────────────────
\c payment_service

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments (
                                        id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id          UUID NOT NULL,
    user_id             UUID NOT NULL,
    parking_id          BIGINT NOT NULL,
    spot_id             BIGINT NOT NULL,
    amount              NUMERIC(10, 2) NOT NULL,
    method              TEXT NOT NULL DEFAULT 'card',
    status              TEXT NOT NULL DEFAULT 'pending',
    provider_payment_id TEXT,
    failure_reason      TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at             TIMESTAMPTZ NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS payments_booking_id_idx ON payments(booking_id);
CREATE INDEX IF NOT EXISTS payments_user_id_idx    ON payments(user_id);
CREATE INDEX IF NOT EXISTS payments_status_idx     ON payments(status);
