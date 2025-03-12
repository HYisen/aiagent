-- This file is designed to be run in migrate procedure. Keep it clean.
-- The whole file is injected into binary code through template during compiling.
-- It's developer's duty to keep the content synced with that in other sql docs.

CREATE TABLE sessions
(
    id      INTEGER PRIMARY KEY ASC,
    name    TEXT NOT NULL,
    user_id INTEGER
) STRICT;

CREATE INDEX idx_sessions_user_id_id ON sessions (user_id, id);

CREATE TABLE chats
(
    id          INTEGER PRIMARY KEY ASC,
    session_id  INTEGER NOT NULL,
    input       TEXT    NOT NULL,
    create_time INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id)
) STRICT;

CREATE INDEX idx_chats_session_id_id ON chats (session_id, id);

CREATE TABLE results
(
    id                      INTEGER PRIMARY KEY ASC,
    chat_id                 INTEGER NOT NULL,
    chat_completion_id      TEXT    NOT NULL, -- UUID of ChatCompletion generated by upstream
    created                 INTEGER NOT NULL,
    model                   TEXT    NOT NULL,
    system_fingerprint      TEXT    NOT NULL,
    finish_reason           TEXT    NOT NULL,

    role                    TEXT    NOT NULL,
    content                 TEXT    NOT NULL,
    reasoning_content       TEXT    NOT NULL,

    prompt_tokens           INTEGER NOT NULL,
    completion_tokens       INTEGER NOT NULL,
    cached_tokens           INTEGER NOT NULL,
    reasoning_tokens        INTEGER NOT NULL,
    prompt_cache_hit_tokens INTEGER NOT NULL,

    FOREIGN KEY (chat_id) REFERENCES chats (id)
) STRICT;