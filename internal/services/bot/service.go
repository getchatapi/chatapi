// Package bot manages registered AI bot participants and their LLM triggering.
package bot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/repository"
)

// Broadcaster is satisfied by realtime.Service. Using an interface keeps the
// bot package free of an import cycle with the realtime package.
type Broadcaster interface {
	BroadcastToRoom(roomID string, message interface{})
}

// chatMessage is an OpenAI-format role/content pair used for history and LLM requests.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Service manages bot registration and LLM triggering.
type Service struct {
	repo        repository.BotRepository
	messageRepo repository.MessageRepository
	httpClient  *http.Client
}

// NewService creates a new bot service.
func NewService(repo repository.BotRepository, messageRepo repository.MessageRepository) *Service {
	return &Service{
		repo:        repo,
		messageRepo: messageRepo,
		httpClient:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// CreateBot registers a new bot.
func (s *Service) CreateBot(req *models.CreateBotRequest) (*models.Bot, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.LLMBaseURL != "" {
		if req.LLMAPIKeyEnv == "" {
			return nil, fmt.Errorf("llm_api_key_env is required when llm_base_url is set")
		}
		if req.Model == "" {
			return nil, fmt.Errorf("model is required when llm_base_url is set")
		}
	}

	bot, err := s.repo.Create(req)
	if err != nil {
		return nil, err
	}

	slog.Info("Created bot", "bot_id", bot.BotID, "name", bot.Name, "managed", bot.LLMBaseURL != "")
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

// TriggerBots finds all managed bots in the room and runs each in its own goroutine.
// It is a no-op if the message sender is itself a bot (bots do not trigger other bots).
func (s *Service) TriggerBots(ctx context.Context, roomID string, msg *models.Message, broadcaster Broadcaster) {
	bots, err := s.repo.GetBotsInRoom(roomID)
	if err != nil {
		slog.Error("TriggerBots: failed to get bots", "room_id", roomID, "error", err)
		return
	}

	for _, b := range bots {
		go s.runBot(b, roomID, msg, broadcaster)
	}
}

// runBot orchestrates a single bot response:
//  1. Fetch recent message history.
//  2. Call system_prompt_webhook (if configured) to get the system prompt.
//  3. Stream the LLM response back via message.stream.* events.
//  4. Persist the final message.
func (s *Service) runBot(b *models.Bot, roomID string, triggerMsg *models.Message, broadcaster Broadcaster) {
	// Use a fresh context so the bot response completes even after the HTTP
	// request that triggered it has returned.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. Fetch the 20 most-recent messages (including the trigger message).
	afterSeq := triggerMsg.Seq - 20
	if afterSeq < 0 {
		afterSeq = 0
	}
	rawHistory, err := s.messageRepo.List(roomID, afterSeq, 20)
	if err != nil {
		slog.Error("runBot: failed to fetch history", "bot_id", b.BotID, "error", err)
		return
	}

	history := make([]chatMessage, 0, len(rawHistory))
	for _, m := range rawHistory {
		role := "user"
		if m.SenderID == b.BotID {
			role = "assistant"
		}
		history = append(history, chatMessage{Role: role, Content: m.Content})
	}

	// 2. Get system prompt from webhook (always called if configured).
	var systemPrompt string
	if b.SystemPromptWebhook != "" {
		systemPrompt, err = s.callWebhook(ctx, b, roomID, triggerMsg, history)
		if err != nil {
			slog.Error("runBot: webhook failed", "bot_id", b.BotID, "error", err)
			return
		}
	}

	// 3. Announce the stream and call the LLM.
	messageID := uuid.New().String()

	broadcaster.BroadcastToRoom(roomID, map[string]interface{}{
		"type":       "message.stream.start",
		"room_id":    roomID,
		"message_id": messageID,
		"sender_id":  b.BotID,
	})

	content, err := s.streamLLM(ctx, b, systemPrompt, history, roomID, messageID, broadcaster)
	if err != nil {
		slog.Error("runBot: LLM streaming failed", "bot_id", b.BotID, "error", err)
		broadcaster.BroadcastToRoom(roomID, map[string]interface{}{
			"type":       "message.stream.error",
			"room_id":    roomID,
			"message_id": messageID,
		})
		return
	}

	if content == "" {
		return
	}

	// 4. Persist the final message.
	stored, err := s.messageRepo.Send(roomID, b.BotID, &models.CreateMessageRequest{Content: content})
	if err != nil {
		slog.Error("runBot: failed to store message", "bot_id", b.BotID, "error", err)
		return
	}

	broadcaster.BroadcastToRoom(roomID, map[string]interface{}{
		"type":       "message.stream.end",
		"room_id":    roomID,
		"message_id": messageID,
		"content":    content,
		"seq":        stored.Seq,
		"sender_id":  b.BotID,
	})
}

// webhookPayload is the payload POSTed to system_prompt_webhook.
type webhookPayload struct {
	BotID   string        `json:"bot_id"`
	RoomID  string        `json:"room_id"`
	Message webhookMsgRef `json:"message"`
	History []chatMessage `json:"history"`
}

type webhookMsgRef struct {
	MessageID string `json:"message_id"`
	SenderID  string `json:"sender_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// webhookResponse is the expected JSON response from system_prompt_webhook.
// The system_prompt is passed verbatim as the system message to the LLM.
type webhookResponse struct {
	SystemPrompt string `json:"system_prompt"`
}

// callWebhook POSTs the incoming message and room history to the bot's
// system_prompt_webhook and returns the system prompt to use for the LLM call.
func (s *Service) callWebhook(ctx context.Context, b *models.Bot, roomID string, msg *models.Message, history []chatMessage) (string, error) {
	payload := webhookPayload{
		BotID:  b.BotID,
		RoomID: roomID,
		Message: webhookMsgRef{
			MessageID: msg.MessageID,
			SenderID:  msg.SenderID,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format(time.RFC3339),
		},
		History: history,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.SystemPromptWebhook, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}

	var result webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode webhook response: %w", err)
	}

	return result.SystemPrompt, nil
}

// openAIRequest is the request body sent to /chat/completions.
type openAIRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// streamChunk is one chunk from the OpenAI SSE stream.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// streamLLM calls the OpenAI-compatible /chat/completions endpoint with
// streaming enabled. Each token is broadcast as a message.stream.delta event.
// Returns the full accumulated content when the stream ends.
func (s *Service) streamLLM(
	ctx context.Context,
	b *models.Bot,
	systemPrompt string,
	history []chatMessage,
	roomID, messageID string,
	broadcaster Broadcaster,
) (string, error) {
	messages := make([]chatMessage, 0, len(history)+1)
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, history...)

	reqBody := openAIRequest{
		Model:    b.Model,
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal LLM request: %w", err)
	}

	baseURL := strings.TrimRight(b.LLMBaseURL, "/")
	endpoint := baseURL + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	if apiKey := os.Getenv(b.LLMAPIKeyEnv); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("LLM returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	// Parse the SSE stream line by line.
	var sb strings.Builder
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

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}

		sb.WriteString(delta)
		broadcaster.BroadcastToRoom(roomID, map[string]interface{}{
			"type":       "message.stream.delta",
			"room_id":    roomID,
			"message_id": messageID,
			"delta":      delta,
		})
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read LLM stream: %w", err)
	}

	return sb.String(), nil
}
