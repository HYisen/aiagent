-- KEEP SYNC with ddl.sql
CREATE TABLE sessions
(
    id        INTEGER PRIMARY KEY ASC,
    name      TEXT    NOT NULL,
    user_id   INTEGER,
    scoped_id INTEGER NOT NULL
) STRICT;

-- KEEP SYNC with ddl.sql
CREATE INDEX idx_sessions_user_id_scoped_id ON sessions (user_id, scoped_id);

INSERT INTO sessions
VALUES (NULL, 'one', NULL, 0),
       (NULL, 'two', NULL, 0),
       (NULL, 'one', NULL, 0),
       (NULL, 'alex', 17, 0),
       (NULL, 'alex_more', 17, 0);;

SELECT id, name, user_id
FROM sessions;