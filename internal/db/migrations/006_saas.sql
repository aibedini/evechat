-- 006_saas.sql — Phase 3 SaaS scaffolding: plans, usage metering, custom domains
CREATE TABLE IF NOT EXISTS plans (
    id             TEXT PRIMARY KEY,
    code           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    price_monthly  INTEGER NOT NULL DEFAULT 0,
    max_agents     INTEGER NOT NULL DEFAULT 3,
    max_conversations_month INTEGER NOT NULL DEFAULT 1000,
    max_ai_tokens_month     INTEGER NOT NULL DEFAULT 100000,
    widget_branding INTEGER NOT NULL DEFAULT 1,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_plans (
    project_id  TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    plan_code   TEXT NOT NULL DEFAULT 'free',
    period_start DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_active   INTEGER NOT NULL DEFAULT 1,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS usage_counters (
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    metric      TEXT NOT NULL,
    period      TEXT NOT NULL,
    value       INTEGER NOT NULL DEFAULT 0,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, metric, period)
);

CREATE TABLE IF NOT EXISTS custom_domains (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    domain     TEXT NOT NULL UNIQUE,
    verified   INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
