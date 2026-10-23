-- 012_push_devices.sql — FCM registration tokens for the Android agent app
CREATE TABLE IF NOT EXISTS push_devices (
    device_id       TEXT PRIMARY KEY,
    agent_id        TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    token           TEXT NOT NULL,
    platform        TEXT NOT NULL DEFAULT '',
    provider        TEXT NOT NULL DEFAULT '',
    app_version     TEXT NOT NULL DEFAULT '',
    app_build       INTEGER NOT NULL DEFAULT 0,
    os_version      TEXT NOT NULL DEFAULT '',
    device_model    TEXT NOT NULL DEFAULT '',
    locale          TEXT NOT NULL DEFAULT '',
    timezone        TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_success_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_push_devices_token ON push_devices(token);
CREATE INDEX IF NOT EXISTS idx_push_devices_agent ON push_devices(agent_id);
CREATE INDEX IF NOT EXISTS idx_push_devices_project ON push_devices(project_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_push_devices_updated ON push_devices(updated_at);
