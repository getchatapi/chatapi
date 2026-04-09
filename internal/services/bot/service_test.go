package bot_test

import (
	"testing"

	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/repository/sqlite"
	"github.com/hastenr/chatapi/internal/services/bot"
	"github.com/hastenr/chatapi/internal/testutil"
)

func newBotSvc(t *testing.T) *bot.Service {
	t.Helper()
	db := testutil.NewTestDB(t)
	return bot.NewService(sqlite.NewBotRepository(db.DB))
}

// --- CreateBot ---

func TestCreateBot(t *testing.T) {
	svc := newBotSvc(t)

	b, err := svc.CreateBot(&models.CreateBotRequest{Name: "Helpful Bot"})
	if err != nil {
		t.Fatalf("CreateBot: %v", err)
	}
	if b.BotID == "" {
		t.Error("BotID is empty")
	}
	if b.Name != "Helpful Bot" {
		t.Errorf("Name = %q, want %q", b.Name, "Helpful Bot")
	}
}

func TestCreateBot_MissingName(t *testing.T) {
	svc := newBotSvc(t)

	_, err := svc.CreateBot(&models.CreateBotRequest{})
	if err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

// --- GetBot ---

func TestGetBot_Found(t *testing.T) {
	svc := newBotSvc(t)

	created, _ := svc.CreateBot(&models.CreateBotRequest{Name: "Finder Bot"})

	got, err := svc.GetBot(created.BotID)
	if err != nil {
		t.Fatalf("GetBot: %v", err)
	}
	if got.BotID != created.BotID {
		t.Errorf("BotID mismatch: got %q want %q", got.BotID, created.BotID)
	}
	if got.Name != "Finder Bot" {
		t.Errorf("Name = %q, want %q", got.Name, "Finder Bot")
	}
}

func TestGetBot_NotFound(t *testing.T) {
	svc := newBotSvc(t)

	_, err := svc.GetBot("nonexistent-bot-id")
	if err == nil {
		t.Error("expected error for missing bot, got nil")
	}
}

// --- ListBots ---

func TestListBots_Empty(t *testing.T) {
	svc := newBotSvc(t)

	bots, err := svc.ListBots()
	if err != nil {
		t.Fatalf("ListBots: %v", err)
	}
	if len(bots) != 0 {
		t.Errorf("ListBots count = %d, want 0", len(bots))
	}
}

func TestListBots_Multiple(t *testing.T) {
	svc := newBotSvc(t)

	for i := 0; i < 3; i++ {
		svc.CreateBot(&models.CreateBotRequest{Name: "Bot"})
	}

	bots, err := svc.ListBots()
	if err != nil {
		t.Fatalf("ListBots: %v", err)
	}
	if len(bots) != 3 {
		t.Errorf("ListBots count = %d, want 3", len(bots))
	}
}

// --- DeleteBot ---

func TestDeleteBot_Existing(t *testing.T) {
	svc := newBotSvc(t)

	b, _ := svc.CreateBot(&models.CreateBotRequest{Name: "Delete Me"})

	if err := svc.DeleteBot(b.BotID); err != nil {
		t.Fatalf("DeleteBot: %v", err)
	}

	_, err := svc.GetBot(b.BotID)
	if err == nil {
		t.Error("expected bot to be gone after delete, but GetBot succeeded")
	}
}

func TestDeleteBot_NotFound(t *testing.T) {
	svc := newBotSvc(t)

	if err := svc.DeleteBot("ghost-id"); err == nil {
		t.Error("expected error deleting nonexistent bot, got nil")
	}
}

// --- IsBot ---

func TestIsBot_True(t *testing.T) {
	svc := newBotSvc(t)

	b, _ := svc.CreateBot(&models.CreateBotRequest{Name: "Am Bot"})

	if !svc.IsBot(b.BotID) {
		t.Errorf("IsBot(%q) = false, want true", b.BotID)
	}
}

func TestIsBot_False(t *testing.T) {
	svc := newBotSvc(t)

	if svc.IsBot("regular-user") {
		t.Error("IsBot(regular-user) = true, want false")
	}
}
