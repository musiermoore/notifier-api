CREATE TABLE IF NOT EXISTS telegram_deliveries (
    id TEXT PRIMARY KEY,
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id TEXT NOT NULL,
    message TEXT NOT NULL,
    deliver_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (item_id)
);

CREATE INDEX IF NOT EXISTS telegram_deliveries_status_deliver_at_idx
    ON telegram_deliveries(status, deliver_at);
