-- KEEP SYNC with ddl.sql
CREATE TABLE users
(
    id                INTEGER PRIMARY KEY ASC,
    nickname          TEXT    NOT NULL,
    sessions_sequence INTEGER NOT NULL
) STRICT;

-- provide id 0 user for compatibility on FK user_id
-- KEEP SYNC with ddl.sql
INSERT INTO users (id, nickname, sessions_sequence)
VALUES (0, 'creator', 1000);