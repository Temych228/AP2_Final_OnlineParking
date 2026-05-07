CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS notifications (
                                             id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ NULL
    );

CREATE TABLE IF NOT EXISTS notification_preferences (
                                                        user_id TEXT PRIMARY KEY,
                                                        email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
                                                        sms_enabled BOOLEAN NOT NULL DEFAULT FALSE,
                                                        push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
                                                        marketing_emails BOOLEAN NOT NULL DEFAULT FALSE,
                                                        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS notifications_user_id_idx ON notifications(user_id);
CREATE INDEX IF NOT EXISTS notifications_is_read_idx ON notifications(is_read);
CREATE INDEX IF NOT EXISTS notifications_created_at_idx ON notifications(created_at DESC);