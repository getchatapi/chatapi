package rest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/hastenr/chatapi/internal/config"
	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/services/chatroom"
	"github.com/hastenr/chatapi/internal/services/delivery"
	"github.com/hastenr/chatapi/internal/services/message"
	"github.com/hastenr/chatapi/internal/services/notification"
	"github.com/hastenr/chatapi/internal/services/realtime"
	"github.com/hastenr/chatapi/internal/services/tenant"
)

// Handler handles REST API requests
type Handler struct {
	tenantSvc   *tenant.Service
	chatroomSvc *chatroom.Service
	messageSvc  *message.Service
	realtimeSvc *realtime.Service
	deliverySvc *delivery.Service
	notifSvc    *notification.Service
	config      *config.Config
	startTime   time.Time
}

// NewHandler creates a new REST handler
func NewHandler(
	tenantSvc *tenant.Service,
	chatroomSvc *chatroom.Service,
	messageSvc *message.Service,
	realtimeSvc *realtime.Service,
	deliverySvc *delivery.Service,
	notifSvc *notification.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		tenantSvc:   tenantSvc,
		chatroomSvc: chatroomSvc,
		messageSvc:  messageSvc,
		realtimeSvc: realtimeSvc,
		deliverySvc: deliverySvc,
		notifSvc:    notifSvc,
		config:      config,
		startTime:   time.Now(),
	}
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// RegisterRoutes registers all REST routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Health check and metrics
	mux.HandleFunc("GET /health", h.HandleHealth)
	mux.HandleFunc("GET /metrics", h.HandleMetrics)

	// Rooms
	mux.HandleFunc("POST /rooms", h.HandleCreateRoom)
	mux.HandleFunc("GET /rooms/{room_id}", h.HandleGetRoom)
	mux.HandleFunc("GET /rooms/{room_id}/members", h.HandleGetRoomMembers)

	// Messages
	mux.HandleFunc("POST /rooms/{room_id}/messages", h.HandleSendMessage)
	mux.HandleFunc("GET /rooms/{room_id}/messages", h.HandleGetMessages)
	mux.HandleFunc("DELETE /rooms/{room_id}/messages/{message_id}", h.HandleDeleteMessage)

	// ACKs
	mux.HandleFunc("POST /acks", h.HandleAck)

	// Notifications
	mux.HandleFunc("POST /notify", h.HandleNotify)

	// Admin
	mux.HandleFunc("POST /admin/tenants", h.HandleCreateTenant)
	mux.HandleFunc("GET /admin/dead-letters", h.HandleGetDeadLetters)
}

// AuthMiddleware for authentication and tenant validation
func (h *Handler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			writeError(w, "unauthorized", "Missing X-API-Key header", http.StatusUnauthorized)
			return
		}

		tenant, err := h.tenantSvc.ValidateAPIKey(apiKey)
		if err != nil {
			writeError(w, "unauthorized", "Invalid API key", http.StatusUnauthorized)
			return
		}

		if err := h.tenantSvc.CheckRateLimit(tenant.TenantID); err != nil {
			w.Header().Set("Retry-After", "60")
			writeError(w, "rate_limit_exceeded", "Too many requests", http.StatusTooManyRequests)
			return
		}

		r.Header.Set("X-Tenant-ID", tenant.TenantID)
		next(w, r)
	}
}

// getTenantID extracts tenant ID from request
func (h *Handler) getTenantID(r *http.Request) string {
	return r.Header.Get("X-Tenant-ID")
}

// getUserID extracts user ID from request
func (h *Handler) getUserID(r *http.Request) string {
	return r.Header.Get("X-User-Id")
}

// requireUserID ensures X-User-Id header is present
func (h *Handler) requireUserID(w http.ResponseWriter, r *http.Request) string {
	userID := h.getUserID(r)
	if userID == "" {
		writeError(w, "invalid_request", "Missing X-User-Id header", http.StatusBadRequest)
		return ""
	}
	return userID
}

// HandleHealth health check endpoint
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(h.startTime)

	// Test database writability
	dbWritable := h.testDatabaseWrite()

	response := map[string]interface{}{
		"status":      "ok",
		"service":     "chatapi",
		"uptime":      uptime.String(),
		"db_writable": dbWritable,
	}

	// Return error status if DB is not writable
	if !dbWritable {
		response["status"] = "error"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleMetrics exposes operational counters for monitoring.
// All values are process-lifetime totals (reset on restart) except active_connections.
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active_connections":   h.realtimeSvc.ActiveConnections(),
		"broadcast_drops":      h.realtimeSvc.DroppedBroadcasts(),
		"messages_sent":        h.messageSvc.MessagesSent(),
		"delivery_attempts":    h.deliverySvc.DeliveryAttempts(),
		"delivery_failures":    h.deliverySvc.DeliveryFailures(),
		"uptime_seconds":       int64(time.Since(h.startTime).Seconds()),
	})
}

// testDatabaseWrite performs a simple write test on the database
func (h *Handler) testDatabaseWrite() bool {
	_, err := h.tenantSvc.ListTenants()
	return err == nil
}

// HandleCreateRoom create room endpoint
func (h *Handler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	var req models.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}

	room, err := h.chatroomSvc.CreateRoom(tenantID, &req)
	if err != nil {
		slog.Error("Failed to create room", "error", err, "tenant_id", tenantID, "user_id", userID)
		writeError(w, "internal_error", "Failed to create room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// HandleGetRoom get room endpoint
func (h *Handler) HandleGetRoom(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	roomID := r.PathValue("room_id")

	room, err := h.chatroomSvc.GetRoom(tenantID, roomID)
	if err != nil {
		writeError(w, "not_found", "Room not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// HandleGetRoomMembers get room members endpoint
func (h *Handler) HandleGetRoomMembers(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	roomID := r.PathValue("room_id")

	members, err := h.chatroomSvc.GetRoomMembers(tenantID, roomID)
	if err != nil {
		writeError(w, "internal_error", "Failed to get room members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": members,
	})
}

// HandleSendMessage send message endpoint
func (h *Handler) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	roomID := r.PathValue("room_id")

	var req models.CreateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		writeError(w, "invalid_request", "content is required", http.StatusBadRequest)
		return
	}

	cfg, err := h.tenantSvc.GetTenantConfig(tenantID)
	if err == nil && cfg.MaxMessageSize > 0 && len(req.Content) > cfg.MaxMessageSize {
		writeError(w, "invalid_request", fmt.Sprintf("content exceeds maximum length of %d characters", cfg.MaxMessageSize), http.StatusBadRequest)
		return
	}

	message, err := h.messageSvc.SendMessage(tenantID, roomID, userID, &req)
	if err != nil {
		slog.Error("Failed to send message", "error", err, "tenant_id", tenantID, "user_id", userID, "room_id", roomID)
		writeError(w, "internal_error", "Failed to send message", http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

// HandleGetMessages get messages endpoint
func (h *Handler) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	roomID := r.PathValue("room_id")

	// Parse query parameters
	afterSeq := 0
	if after := r.URL.Query().Get("after_seq"); after != "" {
		if seq, err := strconv.Atoi(after); err == nil {
			afterSeq = seq
		}
	}

	limit := 50
	if lim := r.URL.Query().Get("limit"); lim != "" {
		if l, err := strconv.Atoi(lim); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	messages, err := h.messageSvc.GetMessages(tenantID, roomID, afterSeq, limit)
	if err != nil {
		writeError(w, "internal_error", "Failed to get messages", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
	})
}

// HandleAck ACK endpoint
func (h *Handler) HandleAck(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	var req models.AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.messageSvc.UpdateLastAck(tenantID, userID, req.RoomID, req.Seq); err != nil {
		writeError(w, "internal_error", "Failed to process acknowledgment", http.StatusInternalServerError)
		return
	}

	// Broadcast ACK to other room members
	h.realtimeSvc.BroadcastToRoom(tenantID, req.RoomID, map[string]interface{}{
		"type":    "ack.received",
		"room_id": req.RoomID,
		"seq":     req.Seq,
		"user_id": userID,
	})

	w.WriteHeader(http.StatusOK)
}

// HandleNotify notify endpoint
func (h *Handler) HandleNotify(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)

	var req models.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}

	notification, err := h.notifSvc.CreateNotification(tenantID, &req)
	if err != nil {
		slog.Error("Failed to create notification", "error", err, "tenant_id", tenantID)
		writeError(w, "internal_error", "Failed to create notification", http.StatusInternalServerError)
		return
	}

	// Deliver immediately to online subscribers; the worker handles retries for the rest.
	go h.deliverySvc.DeliverNow(notification)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(notification)
}

// HandleCreateTenant creates a new tenant (admin only)
func (h *Handler) HandleCreateTenant(w http.ResponseWriter, r *http.Request) {
	masterKey := r.Header.Get("X-Master-Key")
	if masterKey == "" || masterKey != h.config.MasterAPIKey {
		writeError(w, "unauthorized", "Invalid or missing X-Master-Key header", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		writeError(w, "invalid_request", "name is required", http.StatusBadRequest)
		return
	}

	tenant, err := h.tenantSvc.CreateTenant(req.Name)
	if err != nil {
		slog.Error("Failed to create tenant", "error", err)
		writeError(w, "internal_error", "Failed to create tenant", http.StatusInternalServerError)
		return
	}

	// Return tenant details including the plaintext API key.
	// This is the only time the key is returned — it cannot be recovered later.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		TenantID  string `json:"tenant_id"`
		Name      string `json:"name"`
		APIKey    string `json:"api_key"`
		CreatedAt string `json:"created_at"`
	}{
		TenantID:  tenant.TenantID,
		Name:      tenant.Name,
		APIKey:    tenant.APIKey,
		CreatedAt: tenant.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// HandleGetDeadLetters admin endpoint to get failed deliveries
func (h *Handler) HandleGetDeadLetters(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)

	// Parse query parameters
	limit := 100
	if lim := r.URL.Query().Get("limit"); lim != "" {
		if l, err := strconv.Atoi(lim); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Get failed notifications
	failedNotifications, err := h.notifSvc.GetFailedNotifications(tenantID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get failed undelivered messages
	failedMessages, err := h.messageSvc.GetFailedUndeliveredMessages(tenantID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"failed_notifications": failedNotifications,
		"failed_messages":      failedMessages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetUserRooms returns all rooms the authenticated user belongs to
func (h *Handler) HandleGetUserRooms(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	rooms, err := h.chatroomSvc.GetUserRooms(tenantID, userID)
	if err != nil {
		slog.Error("Failed to get user rooms", "error", err, "tenant_id", tenantID, "user_id", userID)
		writeError(w, "internal_error", "Failed to get rooms", http.StatusInternalServerError)
		return
	}

	if rooms == nil {
		rooms = []*models.Room{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"rooms": rooms})
}

// HandleSubscribe subscribes the authenticated user to a notification topic
func (h *Handler) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	var req struct {
		Topic string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Topic == "" {
		writeError(w, "invalid_request", "topic is required", http.StatusBadRequest)
		return
	}

	sub, err := h.notifSvc.Subscribe(tenantID, userID, req.Topic)
	if err != nil {
		slog.Error("Failed to subscribe", "error", err, "tenant_id", tenantID, "user_id", userID)
		writeError(w, "internal_error", "Failed to subscribe", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sub)
}

// HandleUnsubscribe removes a notification subscription by ID
func (h *Handler) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, "invalid_request", "Invalid subscription ID", http.StatusBadRequest)
		return
	}

	if err := h.notifSvc.Unsubscribe(tenantID, userID, id); err != nil {
		writeError(w, "not_found", "Subscription not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListSubscriptions lists the authenticated user's notification subscriptions
func (h *Handler) HandleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	subs, err := h.notifSvc.GetUserSubscriptions(tenantID, userID)
	if err != nil {
		writeError(w, "internal_error", "Failed to get subscriptions", http.StatusInternalServerError)
		return
	}

	if subs == nil {
		subs = []*models.NotificationSubscription{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"subscriptions": subs})
}

// HandleDeleteMessage deletes a message. Only the original sender may delete their own message.
func (h *Handler) HandleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	roomID := r.PathValue("room_id")
	messageID := r.PathValue("message_id")

	seq, err := h.messageSvc.DeleteMessage(tenantID, roomID, messageID, userID)
	if err != nil {
		switch err.Error() {
		case "message not found":
			writeError(w, "not_found", "Message not found", http.StatusNotFound)
		case "forbidden":
			writeError(w, "forbidden", "You can only delete your own messages", http.StatusForbidden)
		default:
			slog.Error("Failed to delete message", "error", err, "tenant_id", tenantID, "message_id", messageID)
			writeError(w, "internal_error", "Failed to delete message", http.StatusInternalServerError)
		}
		return
	}

	h.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
		"type":       "message.deleted",
		"room_id":    roomID,
		"message_id": messageID,
		"seq":        seq,
	})

	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateRoom updates a room's name and/or metadata.
func (h *Handler) HandleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	roomID := r.PathValue("room_id")

	var req models.UpdateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}

	room, err := h.chatroomSvc.UpdateRoom(tenantID, roomID, &req)
	if err != nil {
		if err.Error() == "room not found" {
			writeError(w, "not_found", "Room not found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to update room", "error", err, "tenant_id", tenantID, "room_id", roomID)
		writeError(w, "internal_error", "Failed to update room", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// HandleEditMessage updates the content of a message. Only the original sender may edit.
func (h *Handler) HandleEditMessage(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	roomID := r.PathValue("room_id")
	messageID := r.PathValue("message_id")

	var req models.UpdateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		writeError(w, "invalid_request", "content is required", http.StatusBadRequest)
		return
	}

	msg, err := h.messageSvc.UpdateMessage(tenantID, roomID, messageID, userID, req.Content)
	if err != nil {
		switch err.Error() {
		case "message not found":
			writeError(w, "not_found", "Message not found", http.StatusNotFound)
		case "forbidden":
			writeError(w, "forbidden", "You can only edit your own messages", http.StatusForbidden)
		default:
			slog.Error("Failed to edit message", "error", err, "tenant_id", tenantID, "message_id", messageID)
			writeError(w, "internal_error", "Failed to edit message", http.StatusInternalServerError)
		}
		return
	}

	h.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
		"type":       "message.edited",
		"room_id":    roomID,
		"message_id": messageID,
		"content":    msg.Content,
		"seq":        msg.Seq,
		"sender_id":  msg.SenderID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

// HandleWSToken issues a short-lived WebSocket token for browser clients
func (h *Handler) HandleWSToken(w http.ResponseWriter, r *http.Request) {
	tenantID := h.getTenantID(r)
	userID := h.requireUserID(w, r)
	if userID == "" {
		return
	}

	token := h.realtimeSvc.IssueWSToken(tenantID, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"expires_in": 60,
	})
}
