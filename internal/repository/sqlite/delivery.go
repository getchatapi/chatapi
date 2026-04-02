package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/hastenr/chatapi/internal/models"
)

// SQLiteDeliveryRepository implements repository.DeliveryRepository using SQLite.
type SQLiteDeliveryRepository struct {
	db *sql.DB
}

// NewDeliveryRepository creates a new SQLiteDeliveryRepository.
func NewDeliveryRepository(db *sql.DB) *SQLiteDeliveryRepository {
	return &SQLiteDeliveryRepository{db: db}
}

// GetPendingUndelivered retrieves undelivered messages with fewer than maxAttempts attempts.
// Rows are collected into a slice before returning to avoid holding an open read cursor.
func (r *SQLiteDeliveryRepository) GetPendingUndelivered(tenantID string, maxAttempts, limit int) ([]models.UndeliveredMessage, error) {
	query := `
		SELECT id, tenant_id, user_id, chatroom_id, message_id, seq, attempts
		FROM undelivered_messages
		WHERE tenant_id = ? AND attempts < ?
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, tenantID, maxAttempts, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get undelivered messages: %w", err)
	}

	var pending []models.UndeliveredMessage
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
		)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan undelivered message: %w", err)
		}
		pending = append(pending, msg)
	}
	rows.Close()

	return pending, nil
}

// QueueUndelivered inserts an entry into the undelivered_messages table.
func (r *SQLiteDeliveryRepository) QueueUndelivered(tenantID, userID, roomID, messageID string, seq int) error {
	query := `
		INSERT INTO undelivered_messages (tenant_id, user_id, chatroom_id, message_id, seq)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, tenantID, userID, roomID, messageID, seq)
	return err
}

// MarkMessageDelivered deletes an entry from undelivered_messages.
func (r *SQLiteDeliveryRepository) MarkMessageDelivered(id int) error {
	query := `DELETE FROM undelivered_messages WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// IncrementMessageAttempts increments the attempts counter and sets last_attempt_at.
func (r *SQLiteDeliveryRepository) IncrementMessageAttempts(id int) error {
	query := `
		UPDATE undelivered_messages
		SET attempts = attempts + 1, last_attempt_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query, id)
	return err
}

// GetMessageByID retrieves a message by tenant and message ID.
func (r *SQLiteDeliveryRepository) GetMessageByID(tenantID, messageID string) (*models.Message, error) {
	var msg models.Message
	query := `
		SELECT message_id, tenant_id, chatroom_id, sender_id, seq, content, meta, created_at
		FROM messages
		WHERE tenant_id = ? AND message_id = ?
	`

	err := r.db.QueryRow(query, tenantID, messageID).Scan(
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
		return nil, err
	}

	return &msg, nil
}

// DeleteOldUndelivered deletes undelivered messages that have exceeded maxAttempts and were created before the given time.
func (r *SQLiteDeliveryRepository) DeleteOldUndelivered(tenantID string, maxAttempts int, before time.Time) error {
	query := `
		DELETE FROM undelivered_messages
		WHERE tenant_id = ? AND attempts >= ? AND created_at < ?
	`
	_, err := r.db.Exec(query, tenantID, maxAttempts, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup old undelivered messages: %w", err)
	}
	return nil
}

// GetPendingNotifications retrieves notifications with status pending or processing with fewer than maxAttempts attempts.
func (r *SQLiteDeliveryRepository) GetPendingNotifications(tenantID string, maxAttempts, limit int) ([]models.Notification, error) {
	rows, err := r.db.Query(`
		SELECT notification_id, tenant_id, topic, payload, targets, attempts
		FROM notifications
		WHERE tenant_id = ? AND status IN ('pending', 'processing') AND attempts < ?
		ORDER BY created_at ASC LIMIT ?`,
		tenantID, maxAttempts, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var notif models.Notification
		if err := rows.Scan(
			&notif.NotificationID,
			&notif.TenantID,
			&notif.Topic,
			&notif.Payload,
			&notif.Targets,
			&notif.Attempts,
		); err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifications = append(notifications, notif)
	}

	return notifications, rows.Err()
}

// MarkNotificationDelivered marks a notification as delivered.
func (r *SQLiteDeliveryRepository) MarkNotificationDelivered(notificationID string) error {
	query := `
		UPDATE notifications
		SET status = 'delivered', last_attempt_at = CURRENT_TIMESTAMP
		WHERE notification_id = ?
	`
	_, err := r.db.Exec(query, notificationID)
	return err
}

// DeleteOldNotifications deletes dead notifications created before the given time.
func (r *SQLiteDeliveryRepository) DeleteOldNotifications(tenantID string, before time.Time) error {
	query := `
		DELETE FROM notifications
		WHERE tenant_id = ? AND status = 'dead' AND created_at < ?
	`
	_, err := r.db.Exec(query, tenantID, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup old notifications: %w", err)
	}
	return nil
}

// GetTopicSubscribers returns subscriber IDs for a given topic.
func (r *SQLiteDeliveryRepository) GetTopicSubscribers(tenantID, topic string) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT subscriber_id FROM notification_subscriptions WHERE tenant_id = ? AND topic = ?`,
		tenantID, topic,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("failed to scan subscriber: %w", err)
		}
		subscribers = append(subscribers, uid)
	}

	return subscribers, rows.Err()
}
