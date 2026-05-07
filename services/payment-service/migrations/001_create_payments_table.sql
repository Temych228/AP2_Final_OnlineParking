CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments (
                                        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    booking_id UUID NOT NULL,
    user_id UUID NOT NULL,

    parking_id BIGINT NOT NULL,
    spot_id BIGINT NOT NULL,

    amount NUMERIC(10, 2) NOT NULL,

    method TEXT NOT NULL DEFAULT 'card',
    status TEXT NOT NULL DEFAULT 'pending',

    provider_payment_id TEXT,
    failure_reason TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS payments_booking_id_idx ON payments(booking_id);
CREATE INDEX IF NOT EXISTS payments_user_id_idx ON payments(user_id);
CREATE INDEX IF NOT EXISTS payments_status_idx ON payments(status);