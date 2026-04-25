ALTER TABLE users
    ADD COLUMN IF NOT EXISTS telegram_username TEXT;

CREATE TABLE IF NOT EXISTS telegram_link_codes (
    code TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS telegram_link_codes_user_id_idx ON telegram_link_codes(user_id);
CREATE INDEX IF NOT EXISTS telegram_link_codes_expires_at_idx ON telegram_link_codes(expires_at);
