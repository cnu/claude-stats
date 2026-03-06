package db

// migration1 creates the initial schema.
const migration1 = `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS ingest_meta (
    file_path     TEXT PRIMARY KEY,
    file_size     INTEGER NOT NULL,
    mod_time      INTEGER NOT NULL,
    line_count    INTEGER NOT NULL,
    ingested_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    session_id    TEXT PRIMARY KEY,
    file_path     TEXT NOT NULL,
    project_dir   TEXT,
    project_name  TEXT,
    git_branch    TEXT,
    claude_version TEXT,
    first_message_at INTEGER,
    last_message_at  INTEGER,
    message_count INTEGER DEFAULT 0,
    user_message_count INTEGER DEFAULT 0,
    assistant_message_count INTEGER DEFAULT 0,
    total_input_tokens    INTEGER DEFAULT 0,
    total_output_tokens   INTEGER DEFAULT 0,
    total_cache_create_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens   INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0.0,
    total_duration_ms INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    uuid          TEXT PRIMARY KEY,
    session_id    TEXT NOT NULL,
    parent_uuid   TEXT,
    timestamp     INTEGER NOT NULL,
    role          TEXT,
    model         TEXT,
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_creation_input_tokens INTEGER DEFAULT 0,
    cache_read_input_tokens     INTEGER DEFAULT 0,
    cost_usd      REAL DEFAULT 0.0,
    duration_ms   INTEGER DEFAULT 0,
    content_preview TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

CREATE TABLE IF NOT EXISTS tool_uses (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    message_uuid  TEXT NOT NULL,
    session_id    TEXT NOT NULL,
    tool_name     TEXT NOT NULL,
    tool_input_preview TEXT,
    timestamp     INTEGER NOT NULL,
    FOREIGN KEY (message_uuid) REFERENCES messages(uuid),
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

CREATE TABLE IF NOT EXISTS daily_stats (
    date_key      TEXT PRIMARY KEY,
    session_count INTEGER DEFAULT 0,
    message_count INTEGER DEFAULT 0,
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_create_tokens INTEGER DEFAULT 0,
    cache_read_tokens   INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0.0,
    models_used   TEXT,
    active_minutes INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_model ON messages(model);
CREATE INDEX IF NOT EXISTS idx_messages_role ON messages(role);
CREATE INDEX IF NOT EXISTS idx_tool_uses_name ON tool_uses(tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_uses_session ON tool_uses(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_name);
CREATE INDEX IF NOT EXISTS idx_sessions_first_msg ON sessions(first_message_at);
`
