package message

import (
	"log/slog"
	"sync/atomic"

	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/repository"
)

// Service handles message operations
type Service struct {
	repo         repository.MessageRepository
	messagesSent atomic.Int64
}

// NewService creates a new message service
func NewService(repo repository.MessageRepository) *Service {
	return &Service{repo: repo}
}

// SendMessage stores a message transactionally with sequencing
func (s *Service) SendMessage(tenantID, roomID, senderID string, req *models.CreateMessageRequest) (*models.Message, error) {
	msg, err := s.repo.Send(tenantID, roomID, senderID, req)
	if err != nil {
		return nil, err
	}

	s.messagesSent.Add(1)
	slog.Info("Message sent",
		"tenant_id", tenantID,
		"room_id", roomID,
		"message_id", msg.MessageID,
		"sender_id", senderID,
		"seq", msg.Seq)

	return msg, nil
}

// MessagesSent returns the total number of messages sent since startup.
func (s *Service) MessagesSent() int64 {
	return s.messagesSent.Load()
}

// GetMessages retrieves messages for a room with pagination
func (s *Service) GetMessages(tenantID, roomID string, afterSeq, limit int) ([]*models.Message, error) {
	return s.repo.List(tenantID, roomID, afterSeq, limit)
}

// GetMessage retrieves a single message by ID
func (s *Service) GetMessage(tenantID, messageID string) (*models.Message, error) {
	return s.repo.GetByID(tenantID, messageID)
}

// GetLastAckSeq gets the last acknowledged sequence for a user in a room
func (s *Service) GetLastAckSeq(tenantID, userID, roomID string) (int, error) {
	return s.repo.GetLastAckSeq(tenantID, userID, roomID)
}

// UpdateLastAck updates the last acknowledged sequence for a user in a room
func (s *Service) UpdateLastAck(tenantID, userID, roomID string, seq int) error {
	return s.repo.UpdateLastAck(tenantID, userID, roomID, seq)
}

// QueueUndeliveredMessage queues a message for delivery to offline users
func (s *Service) QueueUndeliveredMessage(tenantID, userID, roomID, messageID string, seq int) error {
	return s.repo.QueueUndelivered(tenantID, userID, roomID, messageID, seq)
}

// GetUndeliveredMessages gets undelivered messages for a user
func (s *Service) GetUndeliveredMessages(tenantID, userID string, limit int) ([]*models.UndeliveredMessage, error) {
	return s.repo.GetUndelivered(tenantID, userID, limit)
}

// MarkMessageDelivered removes a message from the undelivered queue
func (s *Service) MarkMessageDelivered(id int) error {
	return s.repo.MarkDelivered(id)
}

// GetFailedUndeliveredMessages retrieves undelivered messages that have exceeded retry attempts
func (s *Service) GetFailedUndeliveredMessages(tenantID string, limit int) ([]*models.UndeliveredMessage, error) {
	return s.repo.GetFailed(tenantID, limit)
}

// DeleteMessage deletes a message by ID. Only the original sender may delete their message.
// Returns the deleted message's seq number so the caller can broadcast a message.deleted event.
func (s *Service) DeleteMessage(tenantID, roomID, messageID, senderID string) (int, error) {
	seq, err := s.repo.Delete(tenantID, roomID, messageID, senderID)
	if err != nil {
		return 0, err
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
	msg, err := s.repo.Update(tenantID, roomID, messageID, senderID, newContent)
	if err != nil {
		return nil, err
	}

	slog.Info("Message edited",
		"tenant_id", tenantID,
		"room_id", roomID,
		"message_id", messageID,
		"sender_id", senderID)

	return msg, nil
}
