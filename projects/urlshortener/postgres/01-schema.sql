-- Sequence for ID generation (used by app for short code generation)
CREATE SEQUENCE IF NOT EXISTS urls_id_seq;

CREATE TABLE IF NOT EXISTS urls (
    short_code VARCHAR(16) PRIMARY KEY,
    original_url TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
