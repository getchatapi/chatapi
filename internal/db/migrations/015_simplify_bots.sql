-- SQLite does not support DROP COLUMN before 3.35.0, so we recreate the table.
CREATE TABLE IF NOT EXISTS bots_new (
    bot_id     TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO bots_new (bot_id, name, created_at)
SELECT bot_id, name, created_at FROM bots;

DROP TABLE bots;

ALTER TABLE bots_new RENAME TO bots;
