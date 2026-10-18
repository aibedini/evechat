-- 010_channels.sql — inbound/outbound channels (two-way webhooks)
CREATE TABLE IF NOT EXISTS channels (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL DEFAULT 'webhook',
    inbound_secret  TEXT NOT NULL,
    outbound_url    TEXT,
    outbound_secret TEXT,
    mapping         TEXT NOT NULL DEFAULT '{}',
    is_active       INTEGER NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_channels_project ON channels(project_id, created_at DESC);

-- One row per external identity seen on a channel: the bridge between an
-- external user id (Telegram chat, CRM contact, curl test) and a visitor.
CREATE TABLE IF NOT EXISTS channel_identities (
    id          TEXT PRIMARY KEY,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    visitor_id  TEXT NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
    metadata    TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, external_id)
);
CREATE INDEX IF NOT EXISTS idx_channel_identities_visitor ON channel_identities(visitor_id);

-- Raw inbound requests for the inspector. Bodies are scrubbed of secrets and
-- kept for 48 hours, then deleted by the janitor.
CREATE TABLE IF NOT EXISTS webhook_inbox (
    id          TEXT PRIMARY KEY,
    channel_id  TEXT REFERENCES channels(id) ON DELETE SET NULL,
    project_id  TEXT,
    method      TEXT,
    remote_ip   TEXT,
    status      TEXT NOT NULL DEFAULT 'received',
    error       TEXT,
    body        TEXT,
    mapped      TEXT,
    conversation_id TEXT,
    received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_webhook_inbox_channel ON webhook_inbox(channel_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_inbox_time ON webhook_inbox(received_at);
