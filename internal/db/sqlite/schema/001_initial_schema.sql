-- 001_initial_schema.sql
CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    folder TEXT,
    tags TEXT,
    created DATETIME NOT NULL,
    updated DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    source_type TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata TEXT,
    created DATETIME NOT NULL,
    updated DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS vectors (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    text TEXT NOT NULL,
    vector TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT NOT NULL,
    source_ids TEXT
);

CREATE TABLE IF NOT EXISTS relations (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT NOT NULL,
    weight REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
