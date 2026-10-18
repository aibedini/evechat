-- 001_init.sql — core realtime chat schema
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    api_key     TEXT NOT NULL UNIQUE,
    secret_key  TEXT NOT NULL,
    domains     TEXT NOT NULL DEFAULT '[]',
    settings    TEXT NOT NULL DEFAULT '{}',
    is_active   INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agents (
    id                TEXT PRIMARY KEY,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    email             TEXT NOT NULL,
    password_hash     TEXT NOT NULL,
    name              TEXT NOT NULL,
    avatar_url        TEXT,
    role              TEXT NOT NULL DEFAULT 'agent',
    telegram_id       TEXT,
    max_conversations INTEGER NOT NULL DEFAULT 10,
    is_online         INTEGER NOT NULL DEFAULT 0,
    is_active         INTEGER NOT NULL DEFAULT 1,
    last_seen_at      DATETIME,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_email ON agents(project_id, email);
CREATE INDEX IF NOT EXISTS idx_agents_online ON agents(project_id, is_online);

CREATE TABLE IF NOT EXISTS visitors (
    id                     TEXT PRIMARY KEY,
    project_id             TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                   TEXT,
    email                  TEXT,
    phone                  TEXT,
    ip                     TEXT,
    country                TEXT,
    device_type            TEXT,
    os                     TEXT,
    browser                TEXT,
    user_agent             TEXT,
    language               TEXT,
    eve_mobile             TEXT,
    eve_user_id            TEXT,
    eve_reseller_id        TEXT,
    eve_subscription_info  TEXT NOT NULL DEFAULT '{}',
    metadata               TEXT NOT NULL DEFAULT '{}',
    is_banned              INTEGER NOT NULL DEFAULT 0,
    banned_until           DATETIME,
    first_seen_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_visitors_project ON visitors(project_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_visitors_reseller ON visitors(project_id, eve_reseller_id);
CREATE INDEX IF NOT EXISTS idx_visitors_mobile ON visitors(project_id, eve_mobile);

CREATE TABLE IF NOT EXISTS routing_rules (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 0,
    conditions  TEXT NOT NULL DEFAULT '{}',
    route_to    TEXT NOT NULL DEFAULT '{}',
    auto_reply  TEXT,
    is_active   INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_rules_project_prio ON routing_rules(project_id, is_active, priority DESC);

CREATE TABLE IF NOT EXISTS conversations (
    id                 TEXT PRIMARY KEY,
    project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    visitor_id         TEXT NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
    assigned_agent_id  TEXT REFERENCES agents(id) ON DELETE SET NULL,
    routing_rule_id    TEXT REFERENCES routing_rules(id) ON DELETE SET NULL,
    status             TEXT NOT NULL DEFAULT 'pending',
    channel            TEXT NOT NULL DEFAULT 'web',
    subject            TEXT,
    page_url           TEXT,
    page_title         TEXT,
    referrer           TEXT,
    routed_telegram_id TEXT,
    routed_phone       TEXT,
    ticket_id          TEXT,
    unread_agent       INTEGER NOT NULL DEFAULT 0,
    unread_visitor     INTEGER NOT NULL DEFAULT 0,
    last_message_at    DATETIME,
    first_response_at  DATETIME,
    resolved_at        DATETIME,
    csat_score         INTEGER,
    csat_comment       TEXT,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_conv_project_status ON conversations(project_id, status, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_conv_agent ON conversations(assigned_agent_id, status);
CREATE INDEX IF NOT EXISTS idx_conv_visitor ON conversations(visitor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conv_tg ON conversations(routed_telegram_id) WHERE routed_telegram_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_conv_phone ON conversations(routed_phone) WHERE routed_phone IS NOT NULL;

CREATE TABLE IF NOT EXISTS messages (
    id                  TEXT PRIMARY KEY,
    conversation_id     TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    client_message_id   TEXT,
    sender_type         TEXT NOT NULL,
    sender_id           TEXT,
    sender_name         TEXT,
    content_type        TEXT NOT NULL DEFAULT 'text',
    content_encrypted   TEXT NOT NULL,
    content_iv          TEXT NOT NULL,
    key_version         INTEGER NOT NULL DEFAULT 1,
    attachments         TEXT NOT NULL DEFAULT '[]',
    telegram_message_id INTEGER,
    is_internal         INTEGER NOT NULL DEFAULT 0,
    is_read             INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_msg_conv ON messages(conversation_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_dedup ON messages(conversation_id, client_message_id) WHERE client_message_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_msg_tg ON messages(telegram_message_id) WHERE telegram_message_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS visitor_sessions (
    token          TEXT PRIMARY KEY,
    visitor_id     TEXT NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
    project_id     TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    active_conv_id TEXT REFERENCES conversations(id) ON DELETE SET NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at     DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_visitor ON visitor_sessions(visitor_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON visitor_sessions(expires_at);
