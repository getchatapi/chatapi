package sqlite_test

import (
	"strings"
	"testing"

	"github.com/getchatapi/chatapi/internal/models"
	"github.com/getchatapi/chatapi/internal/repository/sqlite"
	"github.com/getchatapi/chatapi/internal/testutil"
)

func newBotRepo(t *testing.T) *sqlite.SQLiteBotRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	return sqlite.NewBotRepository(db.DB)
}

func createBot(t *testing.T, r *sqlite.SQLiteBotRepository, name string) *models.Bot {
	t.Helper()
	bot, err := r.Create(&models.CreateBotRequest{
		Name:         name,
		LLMBaseURL:   "https://api.openai.com/v1",
		LLMAPIKeyEnv: "OPENAI_API_KEY",
		Model:        "gpt-4o",
	})
	if err != nil {
		t.Fatalf("Create bot: %v", err)
	}
	return bot
}

func TestBotRepository_CreateAndGetByID(t *testing.T) {
	r := newBotRepo(t)
	bot := createBot(t, r, "my-bot")

	if bot.BotID == "" {
		t.Error("expected non-empty BotID")
	}
	if bot.Name != "my-bot" {
		t.Errorf("Name: got %q, want %q", bot.Name, "my-bot")
	}

	got, err := r.GetByID(bot.BotID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.BotID != bot.BotID {
		t.Errorf("BotID mismatch: got %q, want %q", got.BotID, bot.BotID)
	}
}

func TestBotRepository_GetByID_NotFound(t *testing.T) {
	r := newBotRepo(t)
	_, err := r.GetByID("no-such-bot")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestBotRepository_List(t *testing.T) {
	r := newBotRepo(t)

	list, err := r.List()
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}

	createBot(t, r, "bot-1")
	createBot(t, r, "bot-2")

	list, err = r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("got %d bots, want 2", len(list))
	}
}

func TestBotRepository_Exists(t *testing.T) {
	r := newBotRepo(t)

	ok, err := r.Exists("nonexistent")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if ok {
		t.Error("expected false for nonexistent bot")
	}

	bot := createBot(t, r, "bot-1")
	ok, err = r.Exists(bot.BotID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Error("expected true for existing bot")
	}
}

func TestBotRepository_Delete(t *testing.T) {
	r := newBotRepo(t)
	bot := createBot(t, r, "bot-1")

	if err := r.Delete(bot.BotID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := r.GetByID(bot.BotID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestBotRepository_Delete_NotFound(t *testing.T) {
	r := newBotRepo(t)
	err := r.Delete("no-such-bot")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestBotRepository_GetBotsInRoom(t *testing.T) {
	db := testutil.NewTestDB(t)
	botRepo := sqlite.NewBotRepository(db.DB)
	roomRepo := sqlite.NewRoomRepository(db.DB)

	// Create room and bots.
	if err := roomRepo.Create(&models.Room{RoomID: "room-1", Type: "group"}); err != nil {
		t.Fatalf("create room: %v", err)
	}
	bot1 := createBot(t, botRepo, "bot-1")
	bot2 := createBot(t, botRepo, "bot-2")

	// Add bot1 to the room as a member, not bot2.
	if err := roomRepo.AddMember("room-1", bot1.BotID); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	bots, err := botRepo.GetBotsInRoom("room-1")
	if err != nil {
		t.Fatalf("GetBotsInRoom: %v", err)
	}
	if len(bots) != 1 {
		t.Fatalf("got %d bots in room, want 1", len(bots))
	}
	if bots[0].BotID != bot1.BotID {
		t.Errorf("got bot %q, want %q", bots[0].BotID, bot1.BotID)
	}

	// Confirm bot2 is not in the room.
	_ = bot2
}
