package sqlite_test

import (
	"strings"
	"testing"

	"github.com/getchatapi/chatapi/internal/models"
	"github.com/getchatapi/chatapi/internal/repository/sqlite"
	"github.com/getchatapi/chatapi/internal/testutil"
)

func newMessageRepo(t *testing.T) (*sqlite.SQLiteRoomRepository, *sqlite.SQLiteMessageRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	return sqlite.NewRoomRepository(db.DB), sqlite.NewMessageRepository(db.DB)
}

func setupRoom(t *testing.T, r *sqlite.SQLiteRoomRepository, roomID string) {
	t.Helper()
	if err := r.Create(&models.Room{RoomID: roomID, Type: "group"}); err != nil {
		t.Fatalf("create room: %v", err)
	}
}

func sendMsg(t *testing.T, m *sqlite.SQLiteMessageRepository, roomID, senderID, content string) *models.Message {
	t.Helper()
	msg, err := m.Send(roomID, senderID, &models.CreateMessageRequest{Content: content})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	return msg
}

func TestMessageRepository_SendAndGetByID(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	msg := sendMsg(t, msgs, "room-1", "alice", "hello")

	if msg.Seq != 1 {
		t.Errorf("first message seq: got %d, want 1", msg.Seq)
	}
	if msg.Content != "hello" {
		t.Errorf("content: got %q, want %q", msg.Content, "hello")
	}

	got, err := msgs.GetByID(msg.MessageID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.MessageID != msg.MessageID {
		t.Errorf("MessageID mismatch")
	}
}

func TestMessageRepository_SequenceIncrements(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	m1 := sendMsg(t, msgs, "room-1", "alice", "first")
	m2 := sendMsg(t, msgs, "room-1", "bob", "second")
	m3 := sendMsg(t, msgs, "room-1", "alice", "third")

	if m1.Seq != 1 || m2.Seq != 2 || m3.Seq != 3 {
		t.Errorf("seq: got %d, %d, %d; want 1, 2, 3", m1.Seq, m2.Seq, m3.Seq)
	}
}

func TestMessageRepository_Send_RoomNotFound(t *testing.T) {
	_, msgs := newMessageRepo(t)
	_, err := msgs.Send("no-room", "alice", &models.CreateMessageRequest{Content: "hi"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestMessageRepository_GetByID_NotFound(t *testing.T) {
	_, msgs := newMessageRepo(t)
	_, err := msgs.GetByID("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestMessageRepository_List(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	for i := 0; i < 5; i++ {
		sendMsg(t, msgs, "room-1", "alice", "msg")
	}

	all, err := msgs.List("room-1", 0, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("got %d messages, want 5", len(all))
	}

	// afterSeq pagination
	after, err := msgs.List("room-1", 3, 10)
	if err != nil {
		t.Fatalf("List afterSeq: %v", err)
	}
	if len(after) != 2 {
		t.Errorf("got %d messages after seq 3, want 2", len(after))
	}
	if after[0].Seq != 4 {
		t.Errorf("first message after seq 3: got seq %d, want 4", after[0].Seq)
	}
}

func TestMessageRepository_Update(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	msg := sendMsg(t, msgs, "room-1", "alice", "original")

	updated, err := msgs.Update("room-1", msg.MessageID, "alice", "edited")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Content != "edited" {
		t.Errorf("content: got %q, want %q", updated.Content, "edited")
	}
}

func TestMessageRepository_Update_Forbidden(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	msg := sendMsg(t, msgs, "room-1", "alice", "original")

	_, err := msgs.Update("room-1", msg.MessageID, "bob", "hack")
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("expected forbidden error, got %v", err)
	}
}

func TestMessageRepository_Delete(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	msg := sendMsg(t, msgs, "room-1", "alice", "bye")

	seq, err := msgs.Delete("room-1", msg.MessageID, "alice")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if seq != msg.Seq {
		t.Errorf("returned seq: got %d, want %d", seq, msg.Seq)
	}

	_, err = msgs.GetByID(msg.MessageID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestMessageRepository_Delete_Forbidden(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	msg := sendMsg(t, msgs, "room-1", "alice", "mine")

	_, err := msgs.Delete("room-1", msg.MessageID, "bob")
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("expected forbidden error, got %v", err)
	}
}

func TestMessageRepository_Ack(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")

	// Initial ack is 0.
	seq, err := msgs.GetLastAckSeq("alice", "room-1")
	if err != nil {
		t.Fatalf("GetLastAckSeq: %v", err)
	}
	if seq != 0 {
		t.Errorf("initial ack: got %d, want 0", seq)
	}

	// Update ack.
	if err := msgs.UpdateLastAck("alice", "room-1", 5); err != nil {
		t.Fatalf("UpdateLastAck: %v", err)
	}
	seq, _ = msgs.GetLastAckSeq("alice", "room-1")
	if seq != 5 {
		t.Errorf("after ack 5: got %d, want 5", seq)
	}

	// Lower ack should not regress.
	msgs.UpdateLastAck("alice", "room-1", 3)
	seq, _ = msgs.GetLastAckSeq("alice", "room-1")
	if seq != 5 {
		t.Errorf("after lower ack: got %d, want 5 (no regression)", seq)
	}
}

func TestMessageRepository_UndeliveredQueue(t *testing.T) {
	rooms, msgs := newMessageRepo(t)
	setupRoom(t, rooms, "room-1")
	sendMsg(t, msgs, "room-1", "alice", "hi")

	if err := msgs.QueueUndelivered("bob", "room-1", "msg-1", 1); err != nil {
		t.Fatalf("QueueUndelivered: %v", err)
	}

	pending, err := msgs.GetUndelivered("bob", 10)
	if err != nil {
		t.Fatalf("GetUndelivered: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("got %d undelivered, want 1", len(pending))
	}

	if err := msgs.MarkDelivered(pending[0].ID); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}

	pending, _ = msgs.GetUndelivered("bob", 10)
	if len(pending) != 0 {
		t.Errorf("got %d undelivered after mark, want 0", len(pending))
	}
}
