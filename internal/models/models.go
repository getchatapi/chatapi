package models

import "time"

// Room represents a chat room
type Room struct {
	RoomID    string    `json:"room_id" db:"room_id"`
	Type      string    `json:"type" db:"type"` // "dm", "group", "channel"
	UniqueKey string    `json:"-" db:"unique_key"` // For DMs
	Name      string    `json:"name,omitempty" db:"name"`
	LastSeq   int       `json:"last_seq" db:"last_seq"`
	Metadata  string    `json:"metadata,omitempty" db:"metadata"` // Arbitrary JSON for app-level context
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RoomMember represents a user's membership in a room
type RoomMember struct {
	ChatroomID string    `json:"chatroom_id" db:"chatroom_id"`
	UserID     string    `json:"user_id" db:"user_id"`
	Role       string    `json:"role" db:"role"`
	JoinedAt   time.Time `json:"joined_at" db:"joined_at"`
}

// Message represents a chat message
type Message struct {
	MessageID  string    `json:"message_id" db:"message_id"`
	ChatroomID string    `json:"chatroom_id" db:"chatroom_id"`
	SenderID   string    `json:"sender_id" db:"sender_id"`
	Seq        int       `json:"seq" db:"seq"`
	Content    string    `json:"content" db:"content"`
	Meta       string    `json:"meta,omitempty" db:"meta"` // JSON metadata
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// DeliveryState tracks per-user per-room delivery state
type DeliveryState struct {
	UserID     string    `json:"user_id" db:"user_id"`
	ChatroomID string    `json:"chatroom_id" db:"chatroom_id"`
	LastAck    int       `json:"last_ack" db:"last_ack"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// UndeliveredMessage represents a message that hasn't been delivered yet
type UndeliveredMessage struct {
	ID            int        `json:"id" db:"id"`
	UserID        string     `json:"user_id" db:"user_id"`
	ChatroomID    string     `json:"chatroom_id" db:"chatroom_id"`
	MessageID     string     `json:"message_id" db:"message_id"`
	Seq           int        `json:"seq" db:"seq"`
	Attempts      int        `json:"attempts" db:"attempts"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty" db:"last_attempt_at"`
}


// Bot represents a registered AI bot participant.
// ChatAPI calls the LLM on the bot's behalf, streams the response back via
// message.stream.* events, and stores the final message.
// LLMAPIKeyEnv names a server-side environment variable — the key is never
// stored in the database.
type Bot struct {
	BotID               string    `json:"bot_id"`
	Name                string    `json:"name"`
	LLMBaseURL          string    `json:"llm_base_url"`
	LLMAPIKeyEnv        string    `json:"llm_api_key_env"`
	Model               string    `json:"model"`
	SystemPromptWebhook string    `json:"system_prompt_webhook,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// CreateBotRequest represents a request to register a bot.
// LLMBaseURL, LLMAPIKeyEnv, and Model are required.
// SystemPromptWebhook is optional — when set, ChatAPI POSTs the incoming
// message and room history to this URL before each LLM call, and uses the
// returned system_prompt as the system message.
type CreateBotRequest struct {
	Name                string `json:"name"`
	LLMBaseURL          string `json:"llm_base_url"`
	LLMAPIKeyEnv        string `json:"llm_api_key_env"`
	Model               string `json:"model"`
	SystemPromptWebhook string `json:"system_prompt_webhook,omitempty"`
}

// AddMemberRequest represents a request to add a member to a room
type AddMemberRequest struct {
	UserID string `json:"user_id"`
}

// UpdateRoomRequest represents a request to update a room's name or metadata.
// Pointer fields: nil means "do not change this field".
type UpdateRoomRequest struct {
	Name     *string `json:"name"`
	Metadata *string `json:"metadata"`
}

// UpdateMessageRequest represents a request to edit a message's content.
type UpdateMessageRequest struct {
	Content string `json:"content"`
}

// CreateRoomRequest represents a request to create a room
type CreateRoomRequest struct {
	Type     string   `json:"type"` // "dm", "group", "channel"
	Members  []string `json:"members"`
	Name     string   `json:"name,omitempty"`
	Metadata string   `json:"metadata,omitempty"` // Arbitrary JSON (listing_id, order_id, etc.)
}

// CreateMessageRequest represents a request to send a message
type CreateMessageRequest struct {
	Content string `json:"content"`
	Meta    string `json:"meta,omitempty"` // JSON metadata
}

// AckRequest represents an acknowledgment of message delivery
type AckRequest struct {
	RoomID string `json:"room_id"`
	Seq    int    `json:"seq"`
}


// WSMessage represents a WebSocket message
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// WSMessageSend represents a send message command
type WSMessageSend struct {
	RoomID  string `json:"room_id"`
	Content string `json:"content"`
	Meta    string `json:"meta,omitempty"`
}

// WSAck represents an acknowledgment
type WSAck struct {
	RoomID string `json:"room_id"`
	Seq    int    `json:"seq"`
}

// WSTyping represents a typing indicator
type WSTyping struct {
	RoomID string `json:"room_id"`
	Action string `json:"action"` // "start" or "stop"
}
