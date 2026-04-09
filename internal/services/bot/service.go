// Package bot manages registered AI bot participants.
package bot

import (
	"fmt"
	"log/slog"

	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/repository"
)

// Service manages bot registration.
type Service struct {
	repo repository.BotRepository
}

// NewService creates a new bot service.
func NewService(repo repository.BotRepository) *Service {
	return &Service{repo: repo}
}

// CreateBot registers a new bot.
func (s *Service) CreateBot(req *models.CreateBotRequest) (*models.Bot, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	bot, err := s.repo.Create(req)
	if err != nil {
		return nil, err
	}

	slog.Info("Created bot", "bot_id", bot.BotID, "name", bot.Name)
	return bot, nil
}

// GetBot retrieves a bot by ID.
func (s *Service) GetBot(botID string) (*models.Bot, error) {
	return s.repo.GetByID(botID)
}

// ListBots returns all registered bots.
func (s *Service) ListBots() ([]*models.Bot, error) {
	return s.repo.List()
}

// DeleteBot removes a bot by ID.
func (s *Service) DeleteBot(botID string) error {
	if err := s.repo.Delete(botID); err != nil {
		return err
	}
	slog.Info("Deleted bot", "bot_id", botID)
	return nil
}

// IsBot reports whether the given user ID belongs to a registered bot.
func (s *Service) IsBot(userID string) bool {
	exists, err := s.repo.Exists(userID)
	return err == nil && exists
}
