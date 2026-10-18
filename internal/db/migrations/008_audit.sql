-- 008_audit.sql — immutable admin action log
CREATE TABLE IF NOT EXISTS audit_log (
    id          TEXT PRIMARY KEY,
    project_id  TEXT,
    actor_type  TEXT NOT NULL,
    actor_id    TEXT,
    actor_name  TEXT,
    action      TEXT NOT NULL,
    target_type TEXT,
    target_id   TEXT,
    ip          TEXT,
    user_agent  TEXT,
    detail      TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_project_time ON audit_log(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action, created_at DESC);
