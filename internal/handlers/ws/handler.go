package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hastenr/chatapi/internal/config"
	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/services/chatroom"
	"github.com/hastenr/chatapi/internal/services/delivery"
	"github.com/hastenr/chatapi/internal/services/message"
	"github.com/hastenr/chatapi/internal/services/realtime"
	"github.com/hastenr/chatapi/internal/services/tenant"
)

// Handler handles WebSocket connections
type Handler struct {
	tenantSvc   *tenant.Service
	chatroomSvc *chatroom.Service
	messageSvc  *message.Service
	realtimeSvc *realtime.Service
	deliverySvc *delivery.Service
	upgrader    websocket.Upgrader
}

// NewHandler creates a new WebSocket handler
func NewHandler(
	tenantSvc *tenant.Service,
	chatroomSvc *chatroom.Service,
	messageSvc *message.Service,
	realtimeSvc *realtime.Service,
	deliverySvc *delivery.Service,
	cfg *config.Config,
) *Handler {
	allowedOrigins := cfg.AllowedOrigins

	if len(allowedOrigins) == 0 {
		slog.Warn("ALLOWED_ORIGINS is not set — WebSocket connections will be rejected for browser clients sending an Origin header. Set to \"*\" to allow all origins (dev only).")
	}

	// Build a fast lookup set
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	checkOrigin := func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Non-browser clients (server-to-server) don't send Origin — allow them.
		if origin == "" {
			return true
		}
		// Wildcard — dev/testing only, logged as a warning at startup above.
		if _, ok := originSet["*"]; ok {
			return true
		}
		if _, ok := originSet[origin]; ok {
			return true
		}
		slog.Warn("WebSocket connection rejected: origin not allowed",
			"origin", origin,
			"remote_addr", r.RemoteAddr)
		return false
	}

	return &Handler{
		tenantSvc:   tenantSvc,
		chatroomSvc: chatroomSvc,
		messageSvc:  messageSvc,
		realtimeSvc: realtimeSvc,
		deliverySvc: deliverySvc,
		upgrader:    websocket.Upgrader{CheckOrigin: checkOrigin},
	}
}

// HandleConnection handles WebSocket connections
func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	var tenantID, userID string

	// Token-based auth — issued by POST /ws/token, used by browser clients
	if token := r.URL.Query().Get("token"); token != "" {
		tid, uid, ok := h.realtimeSvc.ConsumeWSToken(token)
		if !ok {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		tenantID = tid
		userID = uid

		// Still enforce rate limit (need tenant object for ID, which we already have)
		if err := h.tenantSvc.CheckRateLimit(tenantID); err != nil {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	} else {
		// Header-based auth — for server-to-server / Node.js clients
		apiKey := r.Header.Get("X-API-Key")
		userID = r.Header.Get("X-User-Id")
		if userID == "" {
			userID = r.URL.Query().Get("user_id")
		}

		if apiKey == "" || userID == "" {
			http.Error(w, "Missing authentication", http.StatusUnauthorized)
			return
		}

		t, err := h.tenantSvc.ValidateAPIKey(apiKey)
		if err != nil {
			http.Error(w, "Invalid authentication", http.StatusUnauthorized)
			return
		}
		tenantID = t.TenantID

		if err := h.tenantSvc.CheckRateLimit(tenantID); err != nil {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	}

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade connection", "error", err)
		return
	}

	// Register connection — enforces per-user connection cap
	if err := h.realtimeSvc.RegisterConnection(tenantID, userID, conn); err != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "connection limit reached"))
		conn.Close()
		return
	}

	// Send presence update
	h.realtimeSvc.BroadcastPresenceUpdate(tenantID, userID, "online")

	// Handle reconnect sync - send missed messages
	go h.handleReconnectSync(tenantID, userID, conn)

	// Start connection handler
	go h.handleConnection(tenantID, userID, conn)
}

// handleReconnectSync sends missed messages to a reconnecting client
func (h *Handler) handleReconnectSync(tenantID, userID string, conn *websocket.Conn) {
	// Get user's rooms
	// This is a simplified implementation - in practice you'd query the database
	// for rooms the user is a member of

	// For now, we'll skip this and let the client request messages as needed
	// In a full implementation, you'd:
	// 1. Get user's rooms from database
	// 2. For each room, get last_ack
	// 3. Query messages where seq > last_ack
	// 4. Send them in order
}

// handleConnection handles messages from a WebSocket connection
func (h *Handler) handleConnection(tenantID, userID string, conn *websocket.Conn) {
	defer func() {
		h.realtimeSvc.UnregisterConnection(tenantID, userID, conn)
		conn.Close()
	}()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket error", "tenant_id", tenantID, "user_id", userID, "error", err)
			}
			break
		}

		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Parse message
		var wsMsg models.WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			slog.Warn("Invalid WebSocket message", "tenant_id", tenantID, "user_id", userID, "error", err)
			continue
		}

		// Handle message based on type
		if err := h.handleMessage(tenantID, userID, &wsMsg); err != nil {
			slog.Error("Failed to handle WebSocket message",
				"tenant_id", tenantID,
				"user_id", userID,
				"type", wsMsg.Type,
				"error", err)
		}
	}
}

// handleMessage processes different types of WebSocket messages
func (h *Handler) handleMessage(tenantID, userID string, msg *models.WSMessage) error {
	switch msg.Type {
	case "send_message":
		return h.handleSendMessage(tenantID, userID, msg.Data)
	case "ack":
		return h.handleAck(tenantID, userID, msg.Data)
	case "typing.start":
		return h.handleTyping(tenantID, userID, msg.Data, "start")
	case "typing.stop":
		return h.handleTyping(tenantID, userID, msg.Data, "stop")
	case "ping":
		// Application-level keepalive. Receiving this resets the read deadline
		// (handled by the read loop), so no response is needed.
		return nil
	default:
		slog.Warn("Unknown message type", "type", msg.Type, "tenant_id", tenantID, "user_id", userID)
		return nil
	}
}

// handleSendMessage handles message sending via WebSocket
func (h *Handler) handleSendMessage(tenantID, userID string, data interface{}) error {
	msgData, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	roomID, ok := msgData["room_id"].(string)
	if !ok {
		return nil
	}

	content, ok := msgData["content"].(string)
	if !ok {
		return nil
	}

	req := &models.CreateMessageRequest{
		Content: content,
	}

	if meta, ok := msgData["meta"].(string); ok {
		req.Meta = meta
	}

	message, err := h.messageSvc.SendMessage(tenantID, roomID, userID, req)
	if err != nil {
		return err
	}

	// Broadcast to realtime subscribers
	broadcast := map[string]interface{}{
		"type":       "message",
		"room_id":    roomID,
		"seq":        message.Seq,
		"message_id": message.MessageID,
		"sender_id":  message.SenderID,
		"content":    message.Content,
		"created_at": message.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if message.Meta != "" {
		broadcast["meta"] = message.Meta
	}
	h.realtimeSvc.BroadcastToRoom(tenantID, roomID, broadcast)

	go h.deliverySvc.HandleNewMessage(tenantID, roomID, message)

	return nil
}

// handleAck handles acknowledgment of message delivery
func (h *Handler) handleAck(tenantID, userID string, data interface{}) error {
	ackData, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	roomID, ok := ackData["room_id"].(string)
	if !ok {
		return nil
	}

	seqFloat, ok := ackData["seq"].(float64)
	if !ok {
		return nil
	}
	seq := int(seqFloat)

	if err := h.messageSvc.UpdateLastAck(tenantID, userID, roomID, seq); err != nil {
		return err
	}

	// Broadcast ACK to other room members
	h.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
		"type":    "ack.received",
		"room_id": roomID,
		"seq":     seq,
		"user_id": userID,
	})

	return nil
}

// handleTyping handles typing indicators
func (h *Handler) handleTyping(tenantID, userID string, data interface{}, action string) error {
	typingData, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	roomID, ok := typingData["room_id"].(string)
	if !ok {
		return nil
	}

	// Broadcast typing indicator to room members
	h.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
		"type":    "typing",
		"room_id": roomID,
		"user_id": userID,
		"action":  action,
	})

	return nil
}