CREATE TABLE IF NOT EXISTS bots (
    bot_id        TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    mode          TEXT NOT NULL DEFAULT 'llm',  -- 'llm' | 'external'
    provider      TEXT,                          -- 'openai' | 'anthropic'
    base_url      TEXT,                          -- override for openai-compatible endpoints
    model         TEXT,
    api_key       TEXT,
    system_prompt TEXT,
    max_context   INTEGER NOT NULL DEFAULT 20,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
