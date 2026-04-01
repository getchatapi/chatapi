// Package bot manages registered AI bot participants and handles LLM-backed auto-responses.
package bot

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/services/chatroom"
	"github.com/hastenr/chatapi/internal/services/delivery"
	"github.com/hastenr/chatapi/internal/services/message"
	"github.com/hastenr/chatapi/internal/services/realtime"
)

// Service manages bot registration and LLM triggering.
type Service struct {
	db          *sql.DB
	messageSvc  *message.Service
	realtimeSvc *realtime.Service
	chatroomSvc *chatroom.Service
	deliverySvc *delivery.Service
	httpClient  *http.Client
}

// NewService creates a new bot service.
func NewService(
	db *sql.DB,
	messageSvc *message.Service,
	realtimeSvc *realtime.Service,
	chatroomSvc *chatroom.Service,
	deliverySvc *delivery.Service,
) *Service {
	return &Service{
		db:          db,
		messageSvc:  messageSvc,
		realtimeSvc: realtimeSvc,
		chatroomSvc: chatroomSvc,
		deliverySvc: deliverySvc,
		httpClient:  &http.Client{Timeout: 120 * time.Second},
	}
}

// --- CRUD ---

// CreateBot registers a new bot.
func (s *Service) CreateBot(req *models.CreateBotRequest) (*models.Bot, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Mode != "llm" && req.Mode != "external" {
		return nil, fmt.Errorf("mode must be 'llm' or 'external'")
	}
	if req.Mode == "llm" {
		if req.Provider != "openai" && req.Provider != "anthropic" {
			return nil, fmt.Errorf("provider must be 'openai' or 'anthropic'")
		}
		if req.Model == "" {
			return nil, fmt.Errorf("model is required for llm bots")
		}
	}

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

	_, err := s.db.Exec(`
		INSERT INTO bots (bot_id, name, mode, provider, base_url, model, api_key, system_prompt, max_context, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bot.BotID, bot.Name, bot.Mode, bot.Provider, bot.BaseURL,
		bot.Model, bot.APIKey, bot.SystemPrompt, bot.MaxContext, bot.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	slog.Info("Created bot", "bot_id", bot.BotID, "name", bot.Name, "mode", bot.Mode)
	return bot, nil
}

// GetBot retrieves a bot by ID.
func (s *Service) GetBot(botID string) (*models.Bot, error) {
	var bot models.Bot
	var provider, baseURL, model, apiKey, systemPrompt sql.NullString
	err := s.db.QueryRow(`
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

// ListBots returns all registered bots.
func (s *Service) ListBots() ([]*models.Bot, error) {
	rows, err := s.db.Query(`
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

// DeleteBot removes a bot by ID.
func (s *Service) DeleteBot(botID string) error {
	result, err := s.db.Exec(`DELETE FROM bots WHERE bot_id = ?`, botID)
	if err != nil {
		return fmt.Errorf("failed to delete bot: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("bot not found")
	}
	slog.Info("Deleted bot", "bot_id", botID)
	return nil
}

// IsBot reports whether the given user ID belongs to a registered bot.
func (s *Service) IsBot(userID string) bool {
	var id string
	err := s.db.QueryRow(`SELECT bot_id FROM bots WHERE bot_id = ?`, userID).Scan(&id)
	return err == nil
}

// --- Triggering ---

// TriggerBots finds LLM-mode bot members in the room and starts their response goroutines.
// Call this after a message has been stored and broadcast. Safe to call concurrently.
func (s *Service) TriggerBots(tenantID, roomID string, msg *models.Message) {
	// Don't let bots trigger other bots (prevents loops).
	if s.IsBot(msg.SenderID) {
		return
	}

	members, err := s.chatroomSvc.GetRoomMembers(tenantID, roomID)
	if err != nil {
		slog.Error("TriggerBots: failed to get room members", "room_id", roomID, "error", err)
		return
	}

	for _, member := range members {
		if member.UserID == msg.SenderID {
			continue
		}
		bot, err := s.GetBot(member.UserID)
		if err != nil {
			continue // not a bot
		}
		if bot.Mode != "llm" {
			continue // external bots manage themselves
		}
		go s.runBot(bot, tenantID, roomID)
	}
}

// runBot fetches room history, calls the LLM, streams tokens via WebSocket,
// then stores and delivers the final message.
func (s *Service) runBot(bot *models.Bot, tenantID, roomID string) {
	history, err := s.messageSvc.GetMessages(tenantID, roomID, 0, bot.MaxContext)
	if err != nil {
		slog.Error("runBot: failed to get messages", "bot_id", bot.BotID, "error", err)
		return
	}

	streamID := uuid.New().String()
	s.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
		"type":      "message.stream.start",
		"stream_id": streamID,
		"room_id":   roomID,
		"sender_id": bot.BotID,
		"bot_name":  bot.Name,
	})

	tokenCh := make(chan string, 64)
	var streamErr error
	go func() {
		defer close(tokenCh)
		if bot.Provider == "anthropic" {
			streamErr = s.streamAnthropic(bot, history, tokenCh)
		} else {
			streamErr = s.streamOpenAI(bot, history, tokenCh)
		}
	}()

	var sb strings.Builder
	for token := range tokenCh {
		sb.WriteString(token)
		s.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
			"type":      "message.stream.delta",
			"stream_id": streamID,
			"delta":     token,
		})
	}

	if streamErr != nil {
		slog.Error("runBot: LLM call failed", "bot_id", bot.BotID, "error", streamErr)
		s.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
			"type":      "message.stream.error",
			"stream_id": streamID,
			"error":     "LLM call failed",
		})
		return
	}

	content := strings.TrimSpace(sb.String())
	if content == "" {
		return
	}

	stored, err := s.messageSvc.SendMessage(tenantID, roomID, bot.BotID, &models.CreateMessageRequest{
		Content: content,
	})
	if err != nil {
		slog.Error("runBot: failed to store bot reply", "bot_id", bot.BotID, "error", err)
		return
	}

	s.realtimeSvc.BroadcastToRoom(tenantID, roomID, map[string]interface{}{
		"type":       "message.stream.end",
		"stream_id":  streamID,
		"message_id": stored.MessageID,
		"seq":        stored.Seq,
		"sender_id":  stored.SenderID,
		"content":    stored.Content,
		"created_at": stored.CreatedAt.Format(time.RFC3339),
	})

	go s.deliverySvc.HandleNewMessage(tenantID, roomID, stored)
}

// --- LLM providers ---

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildMessages converts room history to LLM message format.
// The bot's own messages become "assistant" turns; all others become "user" turns.
func buildMessages(history []*models.Message, bot *models.Bot) []llmMessage {
	msgs := make([]llmMessage, 0, len(history))
	for _, m := range history {
		role := "user"
		if m.SenderID == bot.BotID {
			role = "assistant"
		}
		msgs = append(msgs, llmMessage{Role: role, Content: m.Content})
	}
	return msgs
}

// streamOpenAI calls an OpenAI-compatible chat completions endpoint with streaming.
// Writes text tokens to tokenCh and returns any error after closing the channel.
func (s *Service) streamOpenAI(bot *models.Bot, history []*models.Message, tokenCh chan<- string) error {
	baseURL := bot.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"

	messages := buildMessages(history, bot)

	body := map[string]interface{}{
		"model":    bot.Model,
		"messages": messages,
		"stream":   true,
	}
	if bot.SystemPrompt != "" {
		body["messages"] = append([]llmMessage{{Role: "system", Content: bot.SystemPrompt}}, messages...)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bot.APIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openai error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			tokenCh <- chunk.Choices[0].Delta.Content
		}
	}
	return scanner.Err()
}

// streamAnthropic calls the Anthropic messages API with streaming.
func (s *Service) streamAnthropic(bot *models.Bot, history []*models.Message, tokenCh chan<- string) error {
	messages := buildMessages(history, bot)

	body := map[string]interface{}{
		"model":      bot.Model,
		"messages":   messages,
		"max_tokens": 4096,
		"stream":     true,
	}
	if bot.SystemPrompt != "" {
		body["system"] = bot.SystemPrompt
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", bot.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("anthropic error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			tokenCh <- event.Delta.Text
		}
	}
	return scanner.Err()
}
