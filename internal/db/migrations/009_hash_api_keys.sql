-- Rename api_key to api_key_hash to reflect that we store a SHA-256 hash,
-- not the plaintext key. Existing rows will have their old values in place;
-- a re-issue of API keys is required for any tenants created before this migration.
ALTER TABLE tenants RENAME COLUMN api_key TO api_key_hash;
