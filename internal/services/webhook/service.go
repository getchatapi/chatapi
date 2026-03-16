package webhook

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Service makes outbound HTTP calls to tenant-configured webhook URLs.
type Service struct {
	client *http.Client
}

// NewService creates a new webhook service.
func NewService() *Service {
	return &Service{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// OfflineMessagePayload is the body POSTed to the webhook URL.
type OfflineMessagePayload struct {
	Event        string          `json:"event"` // always "message.new"
	TenantID     string          `json:"tenant_id"`
	RoomID       string          `json:"room_id"`
	RecipientID  string          `json:"recipient_id"`
	RoomMetadata json.RawMessage `json:"room_metadata,omitempty"`
	Message      MessageInfo     `json:"message"`
}

// MessageInfo contains the fields the receiving app needs to build a notification.
type MessageInfo struct {
	MessageID string    `json:"message_id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	Seq       int       `json:"seq"`
	CreatedAt time.Time `json:"created_at"`
}

// NotifyOfflineUser POSTs an OfflineMessagePayload to webhookURL.
// The call is best-effort — failures are logged but do not affect message delivery.
func (s *Service) NotifyOfflineUser(webhookURL, tenantID, roomID, recipientID, roomMetadata string, msg MessageInfo) {
	if webhookURL == "" {
		return
	}

	payload := OfflineMessagePayload{
		Event:       "message.new",
		TenantID:    tenantID,
		RoomID:      roomID,
		RecipientID: recipientID,
		Message:     msg,
	}

	// Embed raw metadata JSON directly rather than double-encoding it.
	if roomMetadata != "" {
		payload.RoomMetadata = json.RawMessage(roomMetadata)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhook: failed to marshal payload", "error", err)
		return
	}

	resp, err := s.client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook: delivery failed",
			"url", webhookURL,
			"tenant_id", tenantID,
			"recipient_id", recipientID,
			"error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("webhook: non-2xx response",
			"url", webhookURL,
			"status", resp.StatusCode,
			"tenant_id", tenantID,
			"recipient_id", recipientID)
	}
}
