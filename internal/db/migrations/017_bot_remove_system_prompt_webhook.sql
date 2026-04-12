-- SQLite does not support DROP COLUMN before 3.35.0, so we recreate the table.
CREATE TABLE IF NOT EXISTS bots_new (
    bot_id          TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    llm_base_url    TEXT NOT NULL DEFAULT '',
    llm_api_key_env TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO bots_new (bot_id, name, llm_base_url, llm_api_key_env, model, created_at)
SELECT bot_id, name, llm_base_url, llm_api_key_env, model, created_at FROM bots;

DROP TABLE bots;

ALTER TABLE bots_new RENAME TO bots;
