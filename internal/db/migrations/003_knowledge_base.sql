-- 003_knowledge_base.sql — help-center articles used by search and AI answers
CREATE TABLE IF NOT EXISTS knowledge_base (
    id           TEXT PRIMARY KEY,
    project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    slug         TEXT NOT NULL,
    content      TEXT NOT NULL DEFAULT '',
    excerpt      TEXT,
    tags         TEXT NOT NULL DEFAULT '[]',
    is_published INTEGER NOT NULL DEFAULT 1,
    view_count   INTEGER NOT NULL DEFAULT 0,
    created_by   TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kb_project_slug ON knowledge_base(project_id, slug);
CREATE INDEX IF NOT EXISTS idx_kb_project_published ON knowledge_base(project_id, is_published, updated_at DESC);
