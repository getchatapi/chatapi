package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/hastenr/chatapi/internal/models"
)

// SQLiteTenantRepository implements repository.TenantRepository using SQLite.
type SQLiteTenantRepository struct {
	db *sql.DB
}

// NewTenantRepository creates a new SQLiteTenantRepository.
func NewTenantRepository(db *sql.DB) *SQLiteTenantRepository {
	return &SQLiteTenantRepository{db: db}
}

// Create inserts a new tenant record. apiKeyHash is the SHA-256 hex digest of the plaintext key.
func (r *SQLiteTenantRepository) Create(tenantID, apiKeyHash, name, configJSON string) error {
	query := `
		INSERT INTO tenants (tenant_id, api_key_hash, name, config)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, tenantID, apiKeyHash, name, configJSON)
	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}
	return nil
}

// GetByAPIKeyHash retrieves a tenant by the SHA-256 hash of their API key.
func (r *SQLiteTenantRepository) GetByAPIKeyHash(apiKeyHash string) (*models.Tenant, error) {
	var tenant models.Tenant
	query := `
		SELECT tenant_id, name, config, created_at
		FROM tenants
		WHERE api_key_hash = ?
	`

	err := r.db.QueryRow(query, apiKeyHash).Scan(
		&tenant.TenantID,
		&tenant.Name,
		&tenant.Config,
		&tenant.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid API key")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &tenant, nil
}

// GetConfig returns the raw JSON config string for a tenant.
func (r *SQLiteTenantRepository) GetConfig(tenantID string) (string, error) {
	var configJSON string
	query := `SELECT config FROM tenants WHERE tenant_id = ?`

	err := r.db.QueryRow(query, tenantID).Scan(&configJSON)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant config: %w", err)
	}

	return configJSON, nil
}

// List returns all tenants. The API key hash is not included.
func (r *SQLiteTenantRepository) List() ([]*models.Tenant, error) {
	query := `SELECT tenant_id, name, config, created_at FROM tenants ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*models.Tenant
	for rows.Next() {
		var tenant models.Tenant
		err := rows.Scan(
			&tenant.TenantID,
			&tenant.Name,
			&tenant.Config,
			&tenant.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}
		tenants = append(tenants, &tenant)
	}

	return tenants, nil
}
