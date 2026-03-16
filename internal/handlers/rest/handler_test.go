package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hastenr/chatapi/internal/config"
	"github.com/hastenr/chatapi/internal/handlers/rest"
	"github.com/hastenr/chatapi/internal/services/chatroom"
	"github.com/hastenr/chatapi/internal/services/delivery"
	"github.com/hastenr/chatapi/internal/services/message"
	"github.com/hastenr/chatapi/internal/services/notification"
	"github.com/hastenr/chatapi/internal/services/realtime"
	"github.com/hastenr/chatapi/internal/services/tenant"
	"github.com/hastenr/chatapi/internal/testutil"
)

const testMasterKey = "test-master-key"

// newTestHandler returns a REST handler and the tenant service wired to an
// in-memory SQLite database. The realtime service is shut down when the test ends.
func newTestHandler(t *testing.T) (*rest.Handler, *tenant.Service) {
	t.Helper()

	db := testutil.NewTestDB(t)
	cfg := &config.Config{
		MasterAPIKey:     testMasterKey,
		DefaultRateLimit: 100,
	}

	tenantSvc := tenant.NewService(db.DB)
	chatroomSvc := chatroom.NewService(db.DB)
	messageSvc := message.NewService(db.DB)
	realtimeSvc := realtime.NewService(db.DB, 5)
	deliverySvc := delivery.NewService(db.DB, realtimeSvc)
	notifSvc := notification.NewService(db.DB)

	t.Cleanup(func() { realtimeSvc.Shutdown(context.Background()) })

	h := rest.NewHandler(tenantSvc, chatroomSvc, messageSvc, realtimeSvc, deliverySvc, notifSvc, cfg)
	return h, tenantSvc
}

// createTenant is a test helper that creates a tenant and returns its plaintext API key.
func createTenant(t *testing.T, tenantSvc *tenant.Service, name string) (tenantID, apiKey string) {
	t.Helper()
	got, err := tenantSvc.CreateTenant(name)
	if err != nil {
		t.Fatalf("createTenant: %v", err)
	}
	return got.TenantID, got.APIKey
}

// --- Health ---

func TestHandleHealth(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

// --- Tenant creation ---

func TestHandleCreateTenant_MissingMasterKey(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/tenants", jsonBody(`{"name":"acme"}`))
	w := httptest.NewRecorder()
	h.HandleCreateTenant(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateTenant_WrongMasterKey(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/tenants", jsonBody(`{"name":"acme"}`))
	req.Header.Set("X-Master-Key", "wrong")
	w := httptest.NewRecorder()
	h.HandleCreateTenant(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateTenant_MissingName(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/tenants", jsonBody(`{}`))
	req.Header.Set("X-Master-Key", testMasterKey)
	w := httptest.NewRecorder()
	h.HandleCreateTenant(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTenant_Valid(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/tenants", jsonBody(`{"name":"acme"}`))
	req.Header.Set("X-Master-Key", testMasterKey)
	w := httptest.NewRecorder()
	h.HandleCreateTenant(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp struct {
		TenantID string `json:"tenant_id"`
		Name     string `json:"name"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TenantID == "" {
		t.Error("tenant_id is empty")
	}
	if resp.Name != "acme" {
		t.Errorf("name = %q, want %q", resp.Name, "acme")
	}
	if resp.APIKey == "" {
		t.Error("api_key is empty — must be returned on creation")
	}
}

// --- Auth middleware ---

func TestAuthMiddleware_MissingAPIKey(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/rooms", nil)
	w := httptest.NewRecorder()
	h.AuthMiddleware(nopHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidAPIKey(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/rooms", nil)
	req.Header.Set("X-API-Key", "invalid")
	w := httptest.NewRecorder()
	h.AuthMiddleware(nopHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_ValidAPIKey(t *testing.T) {
	h, tenantSvc := newTestHandler(t)
	_, apiKey := createTenant(t, tenantSvc, "test")

	req := httptest.NewRequest(http.MethodPost, "/rooms", nil)
	req.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()

	called := false
	h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if !called {
		t.Error("inner handler not called — valid API key was rejected")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- Room creation ---

func TestHandleCreateRoom_GroupRoom(t *testing.T) {
	h, tenantSvc := newTestHandler(t)
	tenantID, apiKey := createTenant(t, tenantSvc, "test")

	body := `{"type":"group","name":"general","members":["user1","user2"]}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", jsonBody(body))
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-User-Id", "user1")
	req.Header.Set("X-Tenant-ID", tenantID)
	w := httptest.NewRecorder()
	h.HandleCreateRoom(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["room_id"] == "" || resp["room_id"] == nil {
		t.Error("room_id is empty in response")
	}
}

func TestHandleCreateRoom_MissingUserID(t *testing.T) {
	h, tenantSvc := newTestHandler(t)
	tenantID, apiKey := createTenant(t, tenantSvc, "test")

	req := httptest.NewRequest(http.MethodPost, "/rooms", jsonBody(`{"type":"group","name":"general"}`))
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-Tenant-ID", tenantID)
	// No X-User-Id
	w := httptest.NewRecorder()
	h.HandleCreateRoom(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- Message sending ---

func TestHandleSendMessage(t *testing.T) {
	h, tenantSvc := newTestHandler(t)
	tenantID, apiKey := createTenant(t, tenantSvc, "test")

	// Create a room first
	roomReq := httptest.NewRequest(http.MethodPost, "/rooms",
		jsonBody(`{"type":"group","name":"general","members":["user1","user2"]}`))
	roomReq.Header.Set("X-API-Key", apiKey)
	roomReq.Header.Set("X-User-Id", "user1")
	roomReq.Header.Set("X-Tenant-ID", tenantID)
	roomW := httptest.NewRecorder()
	h.HandleCreateRoom(roomW, roomReq)
	if roomW.Code != http.StatusOK {
		t.Fatalf("create room status = %d; body: %s", roomW.Code, roomW.Body.String())
	}

	var roomResp map[string]interface{}
	json.NewDecoder(roomW.Body).Decode(&roomResp)
	roomID := roomResp["room_id"].(string)

	// Send a message using PathValue — we need to use a real mux for path params
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rooms/{room_id}/messages", h.HandleSendMessage)

	msgReq := httptest.NewRequest(http.MethodPost, "/rooms/"+roomID+"/messages",
		jsonBody(`{"content":"hello world"}`))
	msgReq.Header.Set("X-API-Key", apiKey)
	msgReq.Header.Set("X-User-Id", "user1")
	msgReq.Header.Set("X-Tenant-ID", tenantID)
	msgW := httptest.NewRecorder()
	mux.ServeHTTP(msgW, msgReq)

	if msgW.Code != http.StatusOK {
		t.Fatalf("send message status = %d; body: %s", msgW.Code, msgW.Body.String())
	}

	var msgResp map[string]interface{}
	if err := json.NewDecoder(msgW.Body).Decode(&msgResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msgResp["message_id"] == nil {
		t.Error("message_id missing from response")
	}
	if msgResp["content"] != "hello world" {
		t.Errorf("content = %q, want %q", msgResp["content"], "hello world")
	}
}

// --- helpers ---

func jsonBody(s string) *bytes.Buffer {
	return bytes.NewBufferString(s)
}

var nopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})
