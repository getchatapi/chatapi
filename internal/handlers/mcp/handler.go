// Package mcp implements an MCP (Model Context Protocol) server using the HTTP+SSE
// transport. Agents connect via GET /mcp/sse to receive a session endpoint, then POST
// JSON-RPC requests to /mcp/message?sessionId=<id>. Responses are streamed back over
// the SSE connection.
//
// Protocol: https://spec.modelcontextprotocol.io/specification/2024-11-05/
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/hastenr/chatapi/internal/auth"
	"github.com/hastenr/chatapi/internal/services/bot"
	"github.com/hastenr/chatapi/internal/services/chatroom"
	"github.com/hastenr/chatapi/internal/services/delivery"
	"github.com/hastenr/chatapi/internal/services/message"
	"github.com/hastenr/chatapi/internal/services/realtime"
)

const protocolVersion = "2024-11-05"

// rpcRequest is an inbound JSON-RPC 2.0 message from the client.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is an outbound JSON-RPC 2.0 message sent via SSE.
type rpcResponse struct {
	JSONRPC string   `json:"jsonrpc"`
	ID      any      `json:"id"`
	Result  any      `json:"result,omitempty"`
	Error   *rpcErr  `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// session represents one SSE connection from an MCP client.
type session struct {
	id     string
	userID string
	ch     chan string     // JSON-encoded rpcResponse strings to stream to client
	ctx    context.Context
	cancel context.CancelFunc
}

// Handler manages MCP sessions and dispatches tool calls.
type Handler struct {
	mu          sync.Mutex
	sessions    map[string]*session
	messageSvc  *message.Service
	chatroomSvc *chatroom.Service
	realtimeSvc *realtime.Service
	deliverySvc *delivery.Service
	botSvc      *bot.Service
	jwtSecret   string
}

// NewHandler creates a new MCP handler.
func NewHandler(
	messageSvc *message.Service,
	chatroomSvc *chatroom.Service,
	realtimeSvc *realtime.Service,
	deliverySvc *delivery.Service,
	botSvc *bot.Service,
	jwtSecret string,
) *Handler {
	return &Handler{
		sessions:    make(map[string]*session),
		messageSvc:  messageSvc,
		chatroomSvc: chatroomSvc,
		realtimeSvc: realtimeSvc,
		deliverySvc: deliverySvc,
		botSvc:      botSvc,
		jwtSecret:   jwtSecret,
	}
}

func (h *Handler) authenticate(r *http.Request) (string, bool) {
	var tokenStr string
	if t := r.URL.Query().Get("token"); t != "" {
		tokenStr = t
	} else if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
		tokenStr = strings.TrimPrefix(hdr, "Bearer ")
	}
	if tokenStr == "" {
		return "", false
	}
	userID, err := auth.ValidateJWT(h.jwtSecret, tokenStr)
	if err != nil {
		return "", false
	}
	return userID, true
}

// HandleSSE opens an SSE connection and streams MCP responses back to the client.
// The first event is an "endpoint" event telling the client where to POST requests.
func (h *Handler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.authenticate(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	sess := &session{
		id:     uuid.New().String(),
		userID: userID,
		ch:     make(chan string, 64),
		ctx:    ctx,
		cancel: cancel,
	}

	h.mu.Lock()
	h.sessions[sess.id] = sess
	h.mu.Unlock()

	defer func() {
		cancel()
		h.mu.Lock()
		delete(h.sessions, sess.id)
		h.mu.Unlock()
		slog.Info("MCP session closed", "session_id", sess.id, "user_id", userID)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Tell the client which URL to POST requests to.
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp/message?sessionId=%s\n\n", sess.id)
	flusher.Flush()

	slog.Info("MCP session opened", "session_id", sess.id, "user_id", userID)

	for {
		select {
		case data := <-sess.ch:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// HandleMessage receives a JSON-RPC request and dispatches it asynchronously.
// Returns 202 immediately; the response arrives via the SSE stream.
func (h *Handler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	sessID := r.URL.Query().Get("sessionId")

	h.mu.Lock()
	sess, ok := h.sessions[sessID]
	h.mu.Unlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Return 202 immediately; response is sent via SSE.
	w.WriteHeader(http.StatusAccepted)

	// Dispatch in its own goroutine so blocking tools (await_response) don't stall
	// the HTTP handler or other concurrent tool calls.
	go h.dispatch(sess, &req)
}

// dispatch routes a JSON-RPC request to the appropriate handler and sends the response.
func (h *Handler) dispatch(sess *session, req *rpcRequest) {
	var result any
	var rpcError *rpcErr

	switch req.Method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "chatapi", "version": "1.0"},
		}

	case "notifications/initialized":
		return // notification — no response expected

	case "ping":
		result = map[string]any{}

	case "tools/list":
		result = map[string]any{"tools": toolSchemas()}

	case "tools/call":
		result, rpcError = h.callTool(sess, req.Params)

	default:
		rpcError = &rpcErr{Code: -32601, Message: "method not found: " + req.Method}
	}

	h.send(sess, &rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcError,
	})
}

// callTool parses the tool name and arguments and calls the appropriate implementation.
func (h *Handler) callTool(sess *session, params json.RawMessage) (any, *rpcErr) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcErr{Code: -32602, Message: "invalid params"}
	}
	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}

	switch p.Name {
	case "send_message":
		return h.toolSendMessage(sess.userID, p.Arguments)
	case "get_messages":
		return h.toolGetMessages(p.Arguments)
	case "create_room":
		return h.toolCreateRoom(p.Arguments)
	case "is_user_online":
		return h.toolIsUserOnline(p.Arguments)
	default:
		return nil, &rpcErr{Code: -32602, Message: "unknown tool: " + p.Name}
	}
}

// send serializes a response and writes it to the session's SSE channel.
func (h *Handler) send(sess *session, resp *rpcResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("MCP: failed to marshal response", "error", err)
		return
	}
	select {
	case sess.ch <- string(data):
	case <-sess.ctx.Done():
	}
}
