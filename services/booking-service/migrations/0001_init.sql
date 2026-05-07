CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    parking_id BIGINT NOT NULL,
    spot_id BIGINT NOT NULL,
    vehicle_plate TEXT NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    cancel_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at TIMESTAMPTZ NULL,
    CHECK (end_time > start_time)
);

CREATE INDEX IF NOT EXISTS bookings_user_id_idx ON bookings (user_id);
CREATE INDEX IF NOT EXISTS bookings_parking_id_idx ON bookings (parking_id);
CREATE INDEX IF NOT EXISTS bookings_spot_id_idx ON bookings (spot_id);
CREATE INDEX IF NOT EXISTS bookings_status_idx ON bookings (status);
CREATE INDEX IF NOT EXISTS bookings_start_time_idx ON bookings (start_time);
CREATE INDEX IF NOT EXISTS bookings_end_time_idx ON bookings (end_time);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'bookings_spot_time_overlap_excl'
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
