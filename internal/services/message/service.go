package message

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hastenr/chatapi/internal/models"
)

// Service handles message operations
type Service struct {
	db           *sql.DB
	messagesSent atomic.Int64
}

// NewService creates a new message service
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SendMessage stores a message transactionally with sequencing
func (s *Service) SendMessage(tenantID, roomID, senderID string, req *models.CreateMessageRequest) (*models.Message, error) {
	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Increment room sequence
	updateSeqQuery := `
		UPDATE rooms
		SET last_seq = last_seq + 1
		WHERE tenant_id = ? AND room_id = ?
	`

	result, err := tx.Exec(updateSeqQuery, tenantID, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to update room sequence: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("room not found")
	}

	// Get the new sequence number
	var seq int
	getSeqQuery := `
		SELECT last_seq
		FROM rooms
		WHERE tenant_id = ? AND room_id = ?
	`

	err = tx.QueryRow(getSeqQuery, tenantID, roomID).Scan(&seq)
	if err != nil {
		return nil, fmt.Errorf("failed to get sequence number: %w", err)
	}

	// Generate message ID (in production, use UUID)
	messageID := generateMessageID()

	// Prepare metadata JSON
	var metaJSON string
	if req.Meta != "" {
		metaJSON = req.Meta
	}

	// Insert message
	now := time.Now()
	insertQuery := `
		INSERT INTO messages (message_id, tenant_id, chatroom_id, sender_id, seq, content, meta, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(insertQuery, messageID, tenantID, roomID, senderID, seq, req.Content, metaJSON, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	message := &models.Message{
		MessageID:  messageID,
		TenantID:   tenantID,
		ChatroomID: roomID,
		SenderID:   senderID,
		Seq:        seq,
		Content:    req.Content,
		Meta:       metaJSON,
		CreatedAt:  now,
	}

	s.messagesSent.Add(1)
	slog.Info("Message sent",
		"tenant_id", tenantID,
		"room_id", roomID,
		"message_id", messageID,
		"sender_id", senderID,
		"seq", seq)

	return message, nil
}

// MessagesSent returns the total number of messages sent since startup.
func (s *Service) MessagesSent() int64 {
	return s.messagesSent.Load()
}

// GetMessages retrieves messages for a room with pagination
func (s *Service) GetMessages(tenantID, roomID string, afterSeq, limit int) ([]*models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50 // default limit
	}

	query := `
		SELECT message_id, tenant_id, chatroom_id, sender_id, seq, content, meta, created_at
		FROM messages
		WHERE tenant_id = ? AND chatroom_id = ?
	`

	args := []interface{}{tenantID, roomID}

	if afterSeq > 0 {
		query += " AND seq > ?"
		args = append(args, afterSeq)
	}

	query += " ORDER BY seq ASC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(
			&msg.MessageID,
			&msg.TenantID,
			&msg.ChatroomID,
			&msg.SenderID,
			&msg.Seq,
			&msg.Content,
			&msg.Meta,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// GetMessage retrieves a single message by ID
func (s *Service) GetMessage(tenantID, messageID string) (*models.Message, error) {
	var msg models.Message
	query := `
		SELECT message_id, tenant_id, chatroom_id, sender_id, seq, content, meta, created_at
		FROM messages
		WHERE tenant_id = ? AND message_id = ?
	`

	err := s.db.QueryRow(query, tenantID, messageID).Scan(
		&msg.MessageID,
		&msg.TenantID,
		&msg.ChatroomID,
		&msg.SenderID,
		&msg.Seq,
		&msg.Content,
		&msg.Meta,
		&msg.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &msg, nil
}

// GetLastAckSeq gets the last acknowledged sequence for a user in a room
func (s *Service) GetLastAckSeq(tenantID, userID, roomID string) (int, error) {
	var lastAck int
	query := `
		SELECT last_ack
		FROM delivery_state
		WHERE tenant_id = ? AND user_id = ? AND chatroom_id = ?
	`

	err := s.db.QueryRow(query, tenantID, userID, roomID).Scan(&lastAck)
	if err == sql.ErrNoRows {
		// No delivery state exists, return 0
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last ack seq: %w", err)
	}

	return lastAck, nil
}

// UpdateLastAck updates the last acknowledged sequence for a user in a room
func (s *Service) UpdateLastAck(tenantID, userID, roomID string, seq int) error {
	// Only update if the new seq is greater than current last_ack
	query := `
		INSERT INTO delivery_state (tenant_id, user_id, chatroom_id, last_ack, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (tenant_id, user_id, chatroom_id) DO UPDATE SET
			last_ack = CASE WHEN excluded.last_ack > last_ack THEN excluded.last_ack ELSE last_ack END,
			updated_at = CURRENT_TIMESTAMP
		WHERE excluded.last_ack > last_ack
	`

	_, err := s.db.Exec(query, tenantID, userID, roomID, seq)
	if err != nil {
		return fmt.Errorf("failed to update last ack: %w", err)
	}

	slog.Debug("Updated last ack",
		"tenant_id", tenantID,
		"user_id", userID,
		"room_id", roomID,
		"seq", seq)

	return nil
}

// QueueUndeliveredMessage queues a message for delivery to offline users
func (s *Service) QueueUndeliveredMessage(tenantID, userID, roomID, messageID string, seq int) error {
	query := `
		INSERT INTO undelivered_messages (tenant_id, user_id, chatroom_id, message_id, seq)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, tenantID, userID, roomID, messageID, seq)
	if err != nil {
		return fmt.Errorf("failed to queue undelivered message: %w", err)
	}

	return nil
}

// GetUndeliveredMessages gets undelivered messages for a user
func (s *Service) GetUndeliveredMessages(tenantID, userID string, limit int) ([]*models.UndeliveredMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, tenant_id, user_id, chatroom_id, message_id, seq, attempts, created_at, last_attempt_at
		FROM undelivered_messages
		WHERE tenant_id = ? AND user_id = ?
		ORDER BY seq ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, tenantID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get undelivered messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.UndeliveredMessage
	for rows.Next() {
		var msg models.UndeliveredMessage
		err := rows.Scan(
			&msg.ID,
			&msg.TenantID,
			&msg.UserID,
			&msg.ChatroomID,
			&msg.MessageID,
			&msg.Seq,
			&msg.Attempts,
			&msg.CreatedAt,
			&msg.LastAttemptAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan undelivered message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// MarkMessageDelivered removes a message from the undelivered queue
func (s *Service) MarkMessageDelivered(id int) error {
	query := `DELETE FROM undelivered_messages WHERE id = ?`

	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark message delivered: %w", err)
	}

	return nil
}

// GetFailedUndeliveredMessages retrieves undelivered messages that have exceeded retry attempts
func (s *Service) GetFailedUndeliveredMessages(tenantID string, limit int) ([]*models.UndeliveredMessage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT id, tenant_id, user_id, chatroom_id, message_id, seq, attempts, created_at, last_attempt_at
		FROM undelivered_messages
		WHERE tenant_id = ? AND attempts >= 5
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed undelivered messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.UndeliveredMessage
	for rows.Next() {
		var msg models.UndeliveredMessage
		err := rows.Scan(
			&msg.ID,
			&msg.TenantID,
			&msg.UserID,
			&msg.ChatroomID,
			&msg.MessageID,
			&msg.Seq,
			&msg.Attempts,
			&msg.CreatedAt,
			&msg.LastAttemptAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan undelivered message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

// DeleteMessage deletes a message by ID. Only the original sender may delete their message.
// Returns the deleted message's seq number so the caller can broadcast a message.deleted event.
func (s *Service) DeleteMessage(tenantID, roomID, messageID, senderID string) (int, error) {
	var storedSenderID string
	var seq int
	query := `
		SELECT sender_id, seq FROM messages
		WHERE tenant_id = ? AND chatroom_id = ? AND message_id = ?
	`
	err := s.db.QueryRow(query, tenantID, roomID, messageID).Scan(&storedSenderID, &seq)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("message not found")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to look up message: %w", err)
	}
	if storedSenderID != senderID {
		return 0, fmt.Errorf("forbidden")
	}

	_, err = s.db.Exec(
		`DELETE FROM messages WHERE tenant_id = ? AND chatroom_id = ? AND message_id = ?`,
		tenantID, roomID, messageID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete message: %w", err)
	}

	slog.Info("Message deleted",
		"tenant_id", tenantID,
		"room_id", roomID,
		"message_id", messageID,
		"sender_id", senderID,
		"seq", seq)

	return seq, nil
}

// UpdateMessage updates the content of a message. Only the original sender may edit.
// Returns the updated message so the caller can broadcast a message.edited event.
func (s *Service) UpdateMessage(tenantID, roomID, messageID, senderID, newContent string) (*models.Message, error) {
	var storedSenderID string
	err := s.db.QueryRow(
		`SELECT sender_id FROM messages WHERE tenant_id = ? AND chatroom_id = ? AND message_id = ?`,
		tenantID, roomID, messageID,
	).Scan(&storedSenderID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up message: %w", err)
	}
	if storedSenderID != senderID {
		return nil, fmt.Errorf("forbidden")
	}

	_, err = s.db.Exec(
		`UPDATE messages SET content = ? WHERE tenant_id = ? AND chatroom_id = ? AND message_id = ?`,
		newContent, tenantID, roomID, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	slog.Info("Message edited",
		"tenant_id", tenantID,
		"room_id", roomID,
		"message_id", messageID,
		"sender_id", senderID)

	return s.GetMessage(tenantID, messageID)
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return uuid.New().String()
}
