-- 002_tickets.sql — tickets convert from conversations
CREATE TABLE IF NOT EXISTS tickets (
    id                  TEXT PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    conversation_id     TEXT REFERENCES conversations(id) ON DELETE SET NULL,
    title               TEXT NOT NULL,
    description         TEXT,
    status              TEXT NOT NULL DEFAULT 'open',
    priority            TEXT NOT NULL DEFAULT 'normal',
    assigned_agent_id   TEXT REFERENCES agents(id) ON DELETE SET NULL,
    visitor_id          TEXT REFERENCES visitors(id) ON DELETE SET NULL,
    tags                TEXT NOT NULL DEFAULT '[]',
    internal_notes      TEXT NOT NULL DEFAULT '[]',
    due_at              DATETIME,
    resolved_at         DATETIME,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tickets_project_status ON tickets(project_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tickets_conversation ON tickets(conversation_id);
CREATE INDEX IF NOT EXISTS idx_tickets_assignee ON tickets(assigned_agent_id, status);
