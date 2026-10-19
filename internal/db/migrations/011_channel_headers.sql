-- 011_channel_headers.sql — outbound headers and conversation continuation
ALTER TABLE channels ADD COLUMN outbound_headers TEXT NOT NULL DEFAULT '{}';
ALTER TABLE channels ADD COLUMN session_ttl_seconds INTEGER NOT NULL DEFAULT 86400;
