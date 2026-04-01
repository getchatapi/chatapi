package mcp

import (
	"time"

	"github.com/hastenr/chatapi/internal/models"
)

const defaultTenantID = "default"

// toolSchemas returns the MCP tool list payload for tools/list.
func toolSchemas() []map[string]any {
	return []map[string]any{
		{
			"name":        "send_message",
			"description": "Send a message to a room as the authenticated bot/agent.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"room_id": map[string]any{"type": "string", "description": "Target room ID"},
					"content": map[string]any{"type": "string", "description": "Message text"},
				},
				"required": []string{"room_id", "content"},
			},
		},
		{
			"name":        "get_messages",
			"description": "Retrieve messages from a room ordered by sequence number.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"room_id":   map[string]any{"type": "string", "description": "Room ID"},
					"after_seq": map[string]any{"type": "integer", "description": "Return only messages after this seq (default 0)"},
					"limit":     map[string]any{"type": "integer", "description": "Max messages to return (default 50, max 100)"},
				},
				"required": []string{"room_id"},
			},
		},
		{
			"name":        "create_room",
			"description": "Create a new room with the specified members.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":    map[string]any{"type": "string", "description": "Room display name"},
					"members": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User IDs to add (include the bot's own ID to join)"},
					"type":    map[string]any{"type": "string", "description": "Room type: 'group' (default) or 'channel'"},
				},
				"required": []string{"name", "members"},
			},
		},
		{
			"name":        "is_user_online",
			"description": "Check whether a user currently has an active WebSocket connection.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID to check"},
				},
				"required": []string{"user_id"},
			},
		},
	}
}

// toolSendMessage sends a message to a room as the bot.
func (h *Handler) toolSendMessage(senderID string, args map[string]any) (any, *rpcErr) {
	roomID, _ := args["room_id"].(string)
	content, _ := args["content"].(string)
	if roomID == "" || content == "" {
		return nil, &rpcErr{Code: -32602, Message: "room_id and content are required"}
	}

	msg, err := h.messageSvc.SendMessage(defaultTenantID, roomID, senderID, &models.CreateMessageRequest{
		Content: content,
	})
	if err != nil {
		return nil, &rpcErr{Code: -32000, Message: "failed to send message: " + err.Error()}
	}

	h.realtimeSvc.BroadcastToRoom(defaultTenantID, roomID, map[string]any{
		"type":       "message",
		"room_id":    roomID,
		"seq":        msg.Seq,
		"message_id": msg.MessageID,
		"sender_id":  msg.SenderID,
		"content":    msg.Content,
		"created_at": msg.CreatedAt.Format(time.RFC3339),
	})
	go h.deliverySvc.HandleNewMessage(defaultTenantID, roomID, msg)

	return map[string]any{
		"message_id": msg.MessageID,
		"seq":        msg.Seq,
		"created_at": msg.CreatedAt.Format(time.RFC3339),
	}, nil
}

// toolGetMessages retrieves messages from a room.
func (h *Handler) toolGetMessages(args map[string]any) (any, *rpcErr) {
	roomID, _ := args["room_id"].(string)
	if roomID == "" {
		return nil, &rpcErr{Code: -32602, Message: "room_id is required"}
	}

	afterSeq := 0
	if v, ok := args["after_seq"].(float64); ok {
		afterSeq = int(v)
	}

	limit := 50
	if v, ok := args["limit"].(float64); ok && int(v) > 0 && int(v) <= 100 {
		limit = int(v)
	}

	msgs, err := h.messageSvc.GetMessages(defaultTenantID, roomID, afterSeq, limit)
	if err != nil {
		return nil, &rpcErr{Code: -32000, Message: "failed to get messages: " + err.Error()}
	}

	return map[string]any{"messages": msgs}, nil
}

// toolCreateRoom creates a new room.
func (h *Handler) toolCreateRoom(args map[string]any) (any, *rpcErr) {
	name, _ := args["name"].(string)
	if name == "" {
		return nil, &rpcErr{Code: -32602, Message: "name is required"}
	}

	var members []string
	if raw, ok := args["members"].([]any); ok {
		for _, m := range raw {
			if s, ok := m.(string); ok && s != "" {
				members = append(members, s)
			}
		}
	}

	roomType := "group"
	if t, _ := args["type"].(string); t != "" {
		roomType = t
	}

	room, err := h.chatroomSvc.CreateRoom(defaultTenantID, &models.CreateRoomRequest{
		Type:    roomType,
		Name:    name,
		Members: members,
	})
	if err != nil {
		return nil, &rpcErr{Code: -32000, Message: "failed to create room: " + err.Error()}
	}

	return room, nil
}

// toolIsUserOnline checks whether a user is currently connected via WebSocket.
func (h *Handler) toolIsUserOnline(args map[string]any) (any, *rpcErr) {
	userID, _ := args["user_id"].(string)
	if userID == "" {
		return nil, &rpcErr{Code: -32602, Message: "user_id is required"}
	}

	online := h.realtimeSvc.IsUserOnline(defaultTenantID, userID)
	return map[string]any{"user_id": userID, "online": online}, nil
}
