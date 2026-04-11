ALTER TABLE bots ADD COLUMN llm_base_url        TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN llm_api_key_env     TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN model               TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN system_prompt_webhook TEXT NOT NULL DEFAULT '';
