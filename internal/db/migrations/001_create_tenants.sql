-- Create tenants table.
-- ChatAPI is single-tenant per deployment (tenant_id = "default").
-- This table is retained for forward-compatibility if multi-tenancy is ever reintroduced.
CREATE TABLE tenants (
  tenant_id    TEXT PRIMARY KEY,
  api_key_hash TEXT UNIQUE NOT NULL,
  name         TEXT,
  config       JSON,
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
