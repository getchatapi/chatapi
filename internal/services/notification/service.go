package notification

import (
	"log/slog"

	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/repository"
)

// Service handles durable notifications
type Service struct {
	repo repository.NotificationRepository
}

// NewService creates a new notification service
func NewService(repo repository.NotificationRepository) *Service {
	return &Service{repo: repo}
}

// CreateNotification creates a new durable notification
func (s *Service) CreateNotification(tenantID string, req *models.CreateNotificationRequest) (*models.Notification, error) {
	notif, err := s.repo.Create(tenantID, req)
	if err != nil {
		return nil, err
	}
	slog.Info("Created notification", "tenant_id", tenantID, "notification_id", notif.NotificationID, "topic", req.Topic)
	return notif, nil
}

// Subscribe subscribes a user to a notification topic
func (s *Service) Subscribe(tenantID, subscriberID, topic string) (*models.NotificationSubscription, error) {
	return s.repo.Subscribe(tenantID, subscriberID, topic)
}

// Unsubscribe removes a subscription owned by the given user
func (s *Service) Unsubscribe(tenantID, subscriberID string, id int) error {
	return s.repo.Unsubscribe(tenantID, subscriberID, id)
}

// GetUserSubscriptions returns all subscriptions for a user
func (s *Service) GetUserSubscriptions(tenantID, subscriberID string) ([]*models.NotificationSubscription, error) {
	return s.repo.ListSubscriptions(tenantID, subscriberID)
}

// GetFailedNotifications retrieves notifications that have failed delivery
func (s *Service) GetFailedNotifications(tenantID string, limit int) ([]*models.Notification, error) {
	return s.repo.GetFailed(tenantID, limit)
}
