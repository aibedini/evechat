-- 005_fts.sql — full text index over knowledge_base.
--
-- NOTE: messages are encrypted at rest (AES-256-GCM), therefore they cannot be
-- indexed by FTS5. Message search decrypts a bounded window (max 2000 recent
-- messages per conversation scope) in services/search.go instead.
--
-- knowledge_base uses TEXT primary keys so the FTS table is a standalone index
-- kept in sync by triggers (rowid is the implicit SQLite rowid of the source row).
CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_base_fts USING fts5(
    kb_id UNINDEXED,
    project_id UNINDEXED,
    title,
    content,
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS kb_ai AFTER INSERT ON knowledge_base BEGIN
    INSERT INTO knowledge_base_fts(rowid, kb_id, project_id, title, content)
    VALUES (new.rowid, new.id, new.project_id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS kb_ad AFTER DELETE ON knowledge_base BEGIN
    DELETE FROM knowledge_base_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER IF NOT EXISTS kb_au AFTER UPDATE ON knowledge_base BEGIN
    DELETE FROM knowledge_base_fts WHERE rowid = old.rowid;
    INSERT INTO knowledge_base_fts(rowid, kb_id, project_id, title, content)
    VALUES (new.rowid, new.id, new.project_id, new.title, new.content);
END;

INSERT INTO knowledge_base_fts(rowid, kb_id, project_id, title, content)
SELECT rowid, id, project_id, title, content FROM knowledge_base
WHERE rowid NOT IN (SELECT rowid FROM knowledge_base_fts);
