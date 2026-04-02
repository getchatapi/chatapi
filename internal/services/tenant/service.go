package tenant

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/ratelimit"
	"github.com/hastenr/chatapi/internal/repository"
)

// Service handles tenant operations
type Service struct {
	repo             repository.TenantRepository
	rateLimiters     sync.Map // map[string]*ratelimit.TokenBucket
	defaultRateLimit int
}

// TenantConfig represents per-tenant configuration
type TenantConfig struct {
	MaxMessageSize       int    `json:"max_message_size"`
	RetryLimit           int    `json:"retry_limit"`
	DurableNotifications bool   `json:"durable_notifications"`
	RateLimit            int    `json:"rate_limit"` // requests per second
	// WebhookURL is called with a POST when a message arrives for an offline user.
	// Payload: {"event":"message.new","tenant_id","room_id","recipient_id","room_metadata","message":{...}}
	WebhookURL string `json:"webhook_url,omitempty"`
	// WebhookSecret is used to sign webhook payloads with HMAC-SHA256.
	// The signature is sent in the X-ChatAPI-Signature header as "sha256=<hex>".
	// If empty, payloads are sent unsigned.
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

// NewService creates a new tenant service
func NewService(repo repository.TenantRepository) *Service {
	return &Service{
		repo:             repo,
		defaultRateLimit: 100, // requests per second
	}
}

// ValidateAPIKey validates an API key and returns the tenant.
// The plaintext key is hashed before the DB lookup; the hash is never returned to callers.
func (s *Service) ValidateAPIKey(apiKey string) (*models.Tenant, error) {
	tenant, err := s.repo.GetByAPIKeyHash(hashAPIKey(apiKey))
	if err != nil {
		slog.Error("Failed to validate API key", "error", err)
		return nil, err
	}
	return tenant, nil
}

// CreateTenant creates a new tenant with a generated API key
func (s *Service) CreateTenant(name string) (*models.Tenant, error) {
	// Generate tenant ID (UUID)
	tenantID, err := generateTenantID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate tenant ID: %w", err)
	}

	// Generate API key (32-byte random hex)
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Default config
	config := TenantConfig{
		MaxMessageSize:       1000,
		RetryLimit:           5,
		DurableNotifications: true,
		RateLimit:            s.defaultRateLimit,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Insert tenant — store the hash, not the plaintext key
	if err := s.repo.Create(tenantID, hashAPIKey(apiKey), name, string(configJSON)); err != nil {
		slog.Error("Failed to create tenant", "error", err)
		return nil, err
	}

	// APIKey is set to the plaintext value here — this is the only time it is
	// returned. The caller must surface it to the admin; it cannot be recovered.
	tenant := &models.Tenant{
		TenantID: tenantID,
		APIKey:   apiKey,
		Name:     name,
		Config:   string(configJSON),
	}

	slog.Info("Tenant created", "tenant_id", tenantID, "name", name)
	return tenant, nil
}

// generateTenantID generates a unique tenant ID
func generateTenantID() (string, error) {
	h, err := generateRandomHex(8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tenant_%s", h), nil
}

// generateAPIKey generates a random API key
func generateAPIKey() (string, error) {
	return generateRandomHex(32)
}

// generateRandomHex generates a random hex string of given byte length
func generateRandomHex(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashAPIKey returns the SHA-256 hex digest of a plaintext API key.
// API keys are 32-byte random values, so a keyed hash is not required.
func hashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// GetTenantConfig returns the configuration for a tenant
func (s *Service) GetTenantConfig(tenantID string) (*TenantConfig, error) {
	configJSON, err := s.repo.GetConfig(tenantID)
	if err != nil {
		return nil, err
	}

	config := &TenantConfig{
		MaxMessageSize:       4096, // 4KB default
		RetryLimit:           5,
		DurableNotifications: true,
		RateLimit:            s.defaultRateLimit,
	}

	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), config); err != nil {
			slog.Warn("Failed to parse tenant config, using defaults", "tenant_id", tenantID, "error", err)
		}
	}

	return config, nil
}

// CheckRateLimit checks if a tenant is within their rate limit
func (s *Service) CheckRateLimit(tenantID string) error {
	// Get or create rate limiter for this tenant
	rateLimiter, exists := s.rateLimiters.Load(tenantID)
	if !exists {
		config, err := s.GetTenantConfig(tenantID)
		var bucket *ratelimit.TokenBucket
		if err != nil {
			slog.Warn("Failed to get tenant config for rate limiting, using default", "tenant_id", tenantID, "error", err)
			bucket = ratelimit.NewTokenBucket(float64(s.defaultRateLimit), float64(s.defaultRateLimit)/2.0)
		} else {
			bucket = ratelimit.NewTokenBucket(float64(config.RateLimit), float64(config.RateLimit)/2.0)
		}
		s.rateLimiters.Store(tenantID, bucket)
		rateLimiter = bucket
	}

	bucket := rateLimiter.(*ratelimit.TokenBucket)

	if !bucket.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	return nil
}

// ListTenants returns all tenants (admin operation). The API key hash is not included.
func (s *Service) ListTenants() ([]*models.Tenant, error) {
	return s.repo.List()
}
