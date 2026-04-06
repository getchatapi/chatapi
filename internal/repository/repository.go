package repository

import (
	"time"

	"github.com/hastenr/chatapi/internal/models"
)

// RoomRepository handles rooms and membership.
type RoomRepository interface {
	GetByID(tenantID, roomID string) (*models.Room, error)
	GetByUniqueKey(tenantID, uniqueKey string) (*models.Room, error)
	Create(tenantID string, room *models.Room) error
	Update(tenantID, roomID string, req *models.UpdateRoomRequest) error
	GetUserRooms(tenantID, userID string) ([]*models.Room, error)
	AddMember(tenantID, roomID, userID string) error
	AddMembers(tenantID, roomID string, userIDs []string) error
	RemoveMember(tenantID, roomID, userID string) error
	GetMembers(tenantID, roomID string) ([]*models.RoomMember, error)
	GetMemberIDs(tenantID, roomID string) ([]string, error)
}

// MessageRepository handles messages and per-user delivery state.
type MessageRepository interface {
	Send(tenantID, roomID, senderID string, req *models.CreateMessageRequest) (*models.Message, error)
	GetByID(tenantID, messageID string) (*models.Message, error)
	List(tenantID, roomID string, afterSeq, limit int) ([]*models.Message, error)
	Update(tenantID, roomID, messageID, senderID, content string) (*models.Message, error)
	Delete(tenantID, roomID, messageID, senderID string) (int, error)
	GetLastAckSeq(tenantID, userID, roomID string) (int, error)
	UpdateLastAck(tenantID, userID, roomID string, seq int) error
	QueueUndelivered(tenantID, userID, roomID, messageID string, seq int) error
	GetUndelivered(tenantID, userID string, limit int) ([]*models.UndeliveredMessage, error)
	MarkDelivered(id int) error
	GetFailed(tenantID string, limit int) ([]*models.UndeliveredMessage, error)
}

// DeliveryRepository handles the delivery worker's DB operations.
type DeliveryRepository interface {
	GetPendingUndelivered(tenantID string, maxAttempts, limit int) ([]models.UndeliveredMessage, error)
	QueueUndelivered(tenantID, userID, roomID, messageID string, seq int) error
	MarkMessageDelivered(id int) error
	IncrementMessageAttempts(id int) error
	GetMessageByID(tenantID, messageID string) (*models.Message, error)
	DeleteOldUndelivered(tenantID string, maxAttempts int, before time.Time) error
	GetPendingNotifications(tenantID string, maxAttempts, limit int) ([]models.Notification, error)
	MarkNotificationDelivered(notificationID string) error
	DeleteOldNotifications(tenantID string, before time.Time) error
	GetTopicSubscribers(tenantID, topic string) ([]string, error)
}

// NotificationRepository handles notifications and subscriptions.
type NotificationRepository interface {
	Create(tenantID string, req *models.CreateNotificationRequest) (*models.Notification, error)
	GetByID(tenantID, notificationID string) (*models.Notification, error)
	GetFailed(tenantID string, limit int) ([]*models.Notification, error)
	Subscribe(tenantID, subscriberID, topic string) (*models.NotificationSubscription, error)
	Unsubscribe(tenantID, subscriberID string, id int) error
	ListSubscriptions(tenantID, subscriberID string) ([]*models.NotificationSubscription, error)
}

// BotRepository handles bot registration.
type BotRepository interface {
	Create(req *models.CreateBotRequest) (*models.Bot, error)
	GetByID(botID string) (*models.Bot, error)
	List() ([]*models.Bot, error)
	Delete(botID string) error
	Exists(botID string) (bool, error)
}

