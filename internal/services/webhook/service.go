package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Service makes outbound HTTP calls to a configured webhook URL.
type Service struct {
	client *http.Client
}

// NewService creates a new webhook service.
func NewService() *Service {
	return &Service{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// OfflineMessagePayload is the body POSTed when a message arrives for an offline user.
type OfflineMessagePayload struct {
	Type         string          `json:"type"` // "message.offline"
	RoomID       string          `json:"room_id"`
	RecipientID  string          `json:"recipient_id"`
	RoomMetadata json.RawMessage `json:"room_metadata,omitempty"`
	Message      MessageInfo     `json:"message"`
}

// MessageInfo contains the fields the receiving app needs to build a push notification.
type MessageInfo struct {
	MessageID string    `json:"message_id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	Seq       int       `json:"seq"`
	CreatedAt time.Time `json:"created_at"`
}

// NotifyOfflineUser POSTs an OfflineMessagePayload to webhookURL.
// If webhookSecret is non-empty the request body is signed with HMAC-SHA256
// and the signature is sent in the X-ChatAPI-Signature header as "sha256=<hex>".
// The call is best-effort — failures are logged but do not affect message delivery.
func (s *Service) NotifyOfflineUser(webhookURL, webhookSecret, roomID, recipientID, roomMetadata string, msg MessageInfo) {
	if webhookURL == "" {
		return
	}

	payload := OfflineMessagePayload{
		Type:        "message.offline",
		RoomID:      roomID,
		RecipientID: recipientID,
		Message:     msg,
	}
	if roomMetadata != "" {
		payload.RoomMetadata = json.RawMessage(roomMetadata)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhook: failed to marshal payload", "error", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		slog.Error("webhook: failed to build request", "url", webhookURL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	sign(req, body, webhookSecret)

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("webhook: delivery failed", "url", webhookURL, "recipient_id", recipientID, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("webhook: non-2xx response", "url", webhookURL, "status", resp.StatusCode, "recipient_id", recipientID)
	}
}

// Post sends a JSON payload to webhookURL and returns the response body.
// Used for synchronous webhook calls that expect a response (e.g. bot.context).
func (s *Service) Post(webhookURL, webhookSecret string, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	sign(req, body, webhookSecret)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// sign adds an HMAC-SHA256 signature header if secret is non-empty.
func sign(req *http.Request, body []byte, secret string) {
	if secret == "" {
		return
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	req.Header.Set("X-ChatAPI-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
}
