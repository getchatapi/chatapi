package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hastenr/chatapi/internal/models"
)

// SQLiteBotRepository implements repository.BotRepository using SQLite.
type SQLiteBotRepository struct {
	db *sql.DB
}

// NewBotRepository creates a new SQLiteBotRepository.
func NewBotRepository(db *sql.DB) *SQLiteBotRepository {
	return &SQLiteBotRepository{db: db}
}

// Create registers a new bot.
func (r *SQLiteBotRepository) Create(req *models.CreateBotRequest) (*models.Bot, error) {
	maxContext := req.MaxContext
	if maxContext <= 0 {
		maxContext = 20
	}

	bot := &models.Bot{
		BotID:        uuid.New().String(),
		Name:         req.Name,
		Mode:         req.Mode,
		Provider:     req.Provider,
		BaseURL:      req.BaseURL,
		Model:        req.Model,
		APIKey:       req.APIKey,
		SystemPrompt: req.SystemPrompt,
		MaxContext:   maxContext,
		CreatedAt:    time.Now().UTC(),
	}

	_, err := r.db.Exec(`
		INSERT INTO bots (bot_id, name, mode, provider, base_url, model, api_key, system_prompt, max_context, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bot.BotID, bot.Name, bot.Mode, bot.Provider, bot.BaseURL,
		bot.Model, bot.APIKey, bot.SystemPrompt, bot.MaxContext, bot.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return bot, nil
}

// GetByID retrieves a bot by ID.
func (r *SQLiteBotRepository) GetByID(botID string) (*models.Bot, error) {
	var bot models.Bot
	var provider, baseURL, model, apiKey, systemPrompt sql.NullString
	err := r.db.QueryRow(`
		SELECT bot_id, name, mode, provider, base_url, model, api_key, system_prompt, max_context, created_at
		FROM bots WHERE bot_id = ?`, botID,
	).Scan(
		&bot.BotID, &bot.Name, &bot.Mode,
		&provider, &baseURL, &model, &apiKey, &systemPrompt,
		&bot.MaxContext, &bot.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bot not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get bot: %w", err)
	}
	bot.Provider = provider.String
	bot.BaseURL = baseURL.String
	bot.Model = model.String
	bot.APIKey = apiKey.String
	bot.SystemPrompt = systemPrompt.String
	return &bot, nil
}

// List returns all registered bots.
func (r *SQLiteBotRepository) List() ([]*models.Bot, error) {
	rows, err := r.db.Query(`
		SELECT bot_id, name, mode, provider, base_url, model, api_key, system_prompt, max_context, created_at
		FROM bots ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list bots: %w", err)
	}
	defer rows.Close()

	var bots []*models.Bot
	for rows.Next() {
		var bot models.Bot
		var provider, baseURL, model, apiKey, systemPrompt sql.NullString
		if err := rows.Scan(
			&bot.BotID, &bot.Name, &bot.Mode,
			&provider, &baseURL, &model, &apiKey, &systemPrompt,
			&bot.MaxContext, &bot.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan bot: %w", err)
		}
		bot.Provider = provider.String
		bot.BaseURL = baseURL.String
		bot.Model = model.String
		bot.APIKey = apiKey.String
		bot.SystemPrompt = systemPrompt.String
		bots = append(bots, &bot)
	}
	return bots, rows.Err()
}

// Delete removes a bot by ID.
func (r *SQLiteBotRepository) Delete(botID string) error {
	result, err := r.db.Exec(`DELETE FROM bots WHERE bot_id = ?`, botID)
	if err != nil {
		return fmt.Errorf("failed to delete bot: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("bot not found")
	}
	return nil
}

// Exists reports whether the given bot ID belongs to a registered bot.
func (r *SQLiteBotRepository) Exists(botID string) (bool, error) {
	var id string
	err := r.db.QueryRow(`SELECT bot_id FROM bots WHERE bot_id = ?`, botID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
