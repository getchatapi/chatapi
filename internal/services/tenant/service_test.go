package tenant_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/hastenr/chatapi/internal/services/tenant"
	"github.com/hastenr/chatapi/internal/testutil"
)

func TestCreateTenant_ReturnsPlaintextKey(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	got, err := svc.CreateTenant("acme")
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if got.TenantID == "" {
		t.Error("TenantID is empty")
	}
	if got.Name != "acme" {
		t.Errorf("Name = %q, want %q", got.Name, "acme")
	}
	if got.APIKey == "" {
		t.Error("APIKey is empty — plaintext key must be returned at creation time")
	}
}

func TestCreateTenant_KeyHashedInDB(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	created, err := svc.CreateTenant("test")
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	var storedValue string
	err = db.DB.QueryRow("SELECT api_key_hash FROM tenants WHERE tenant_id = ?", created.TenantID).Scan(&storedValue)
	if err != nil {
		t.Fatalf("failed to query DB: %v", err)
	}

	if storedValue == created.APIKey {
		t.Error("plaintext API key is stored in DB — expected SHA-256 hash")
	}

	sum := sha256.Sum256([]byte(created.APIKey))
	expectedHash := hex.EncodeToString(sum[:])
	if storedValue != expectedHash {
		t.Errorf("stored value %q is not SHA-256 of plaintext key", storedValue)
	}
}

func TestValidateAPIKey_ValidKey(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	created, err := svc.CreateTenant("test")
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	got, err := svc.ValidateAPIKey(created.APIKey)
	if err != nil {
		t.Fatalf("ValidateAPIKey() error = %v", err)
	}
	if got.TenantID != created.TenantID {
		t.Errorf("TenantID = %q, want %q", got.TenantID, created.TenantID)
	}
}

func TestValidateAPIKey_InvalidKey(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	_, err := svc.ValidateAPIKey("not-a-real-key")
	if err == nil {
		t.Error("ValidateAPIKey() with unknown key: want error, got nil")
	}
}

func TestValidateAPIKey_WrongKey(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	if _, err := svc.CreateTenant("test"); err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	// Correct format, wrong value
	_, err := svc.ValidateAPIKey("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil {
		t.Error("ValidateAPIKey() with wrong key: want error, got nil")
	}
}

func TestValidateAPIKey_DoesNotReturnHash(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	created, _ := svc.CreateTenant("test")
	got, err := svc.ValidateAPIKey(created.APIKey)
	if err != nil {
		t.Fatalf("ValidateAPIKey() error = %v", err)
	}
	// APIKey on a validated tenant should not be populated (hash is never returned)
	if got.APIKey != "" {
		t.Errorf("ValidateAPIKey() returned APIKey = %q; hash must not be exposed", got.APIKey)
	}
}

func TestCheckRateLimit_AllowsFirstRequest(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := tenant.NewService(db.DB)

	created, err := svc.CreateTenant("test")
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	if err := svc.CheckRateLimit(created.TenantID); err != nil {
		t.Errorf("CheckRateLimit() first call = %v, want nil", err)
	}
}
