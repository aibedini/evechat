-- 009_security.sql — visitor reputation and password reset tokens
ALTER TABLE visitors ADD COLUMN reputation INTEGER NOT NULL DEFAULT 100;

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token_hash TEXT PRIMARY KEY,
    agent_id   TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_reset_agent ON password_reset_tokens(agent_id, created_at DESC);
