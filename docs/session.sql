DROP TABLE sessions;

CREATE TABLE sessions
(
    id      INTEGER PRIMARY KEY ASC,
    name    TEXT NOT NULL,
    user_id INTEGER
) STRICT;

CREATE INDEX idx_sessions_user_id_id ON sessions (user_id, id);

INSERT INTO sessions
VALUES (NULL, 'one', NULL),
       (NULL, 'two', NULL),
       (NULL, 'one', NULL),
       (NULL, 'alex', 17),
       (NULL, 'alex_more', 17);;

SELECT id, name, user_id
FROM sessions;
