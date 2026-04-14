package sqlite_test

import (
	"strings"
	"testing"

	"github.com/getchatapi/chatapi/internal/models"
	"github.com/getchatapi/chatapi/internal/repository/sqlite"
	"github.com/getchatapi/chatapi/internal/testutil"
)

func newRoomRepo(t *testing.T) *sqlite.SQLiteRoomRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	return sqlite.NewRoomRepository(db.DB)
}

func insertRoom(t *testing.T, r *sqlite.SQLiteRoomRepository, roomID, typ string) {
	t.Helper()
	if err := r.Create(&models.Room{RoomID: roomID, Type: typ}); err != nil {
		t.Fatalf("insert room: %v", err)
	}
}

func TestRoomRepository_CreateAndGetByID(t *testing.T) {
	r := newRoomRepo(t)

	if err := r.Create(&models.Room{
		RoomID: "room-1",
		Type:   "group",
		Name:   "general",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	room, err := r.GetByID("room-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if room.RoomID != "room-1" {
		t.Errorf("RoomID: got %q, want %q", room.RoomID, "room-1")
	}
	if room.Name != "general" {
		t.Errorf("Name: got %q, want %q", room.Name, "general")
	}
}

func TestRoomRepository_GetByID_NotFound(t *testing.T) {
	r := newRoomRepo(t)
	_, err := r.GetByID("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestRoomRepository_CreateWithUniqueKey(t *testing.T) {
	r := newRoomRepo(t)

	if err := r.Create(&models.Room{
		RoomID:    "dm-1",
		Type:      "dm",
		UniqueKey: "dm:alice:bob",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	room, err := r.GetByUniqueKey("dm:alice:bob")
	if err != nil {
		t.Fatalf("GetByUniqueKey: %v", err)
	}
	if room == nil || room.RoomID != "dm-1" {
		t.Errorf("expected room dm-1, got %v", room)
	}
}

func TestRoomRepository_GetByUniqueKey_NotFound(t *testing.T) {
	r := newRoomRepo(t)
	room, err := r.GetByUniqueKey("no:such:key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if room != nil {
		t.Error("expected nil for missing unique key")
	}
}

func TestRoomRepository_Update(t *testing.T) {
	r := newRoomRepo(t)
	insertRoom(t, r, "room-1", "group")

	name := "updated"
	meta := `{"color":"blue"}`
	if err := r.Update("room-1", &models.UpdateRoomRequest{Name: &name, Metadata: &meta}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	room, _ := r.GetByID("room-1")
	if room.Name != name {
		t.Errorf("Name: got %q, want %q", room.Name, name)
	}
	if room.Metadata != meta {
		t.Errorf("Metadata: got %q, want %q", room.Metadata, meta)
	}
}

func TestRoomRepository_Update_NotFound(t *testing.T) {
	r := newRoomRepo(t)
	name := "x"
	err := r.Update("no-room", &models.UpdateRoomRequest{Name: &name})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestRoomRepository_Members(t *testing.T) {
	r := newRoomRepo(t)
	insertRoom(t, r, "room-1", "group")

	if err := r.AddMember("room-1", "alice"); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := r.AddMembers("room-1", []string{"bob", "carol"}); err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	members, err := r.GetMembers("room-1")
	if err != nil {
		t.Fatalf("GetMembers: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("got %d members, want 3", len(members))
	}

	ids, err := r.GetMemberIDs("room-1")
	if err != nil {
		t.Fatalf("GetMemberIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("got %d member IDs, want 3", len(ids))
	}
}

func TestRoomRepository_AddMembers_Idempotent(t *testing.T) {
	r := newRoomRepo(t)
	insertRoom(t, r, "room-1", "group")

	r.AddMembers("room-1", []string{"alice", "bob"})
	// Re-adding the same members should not error or duplicate.
	if err := r.AddMembers("room-1", []string{"alice", "bob"}); err != nil {
		t.Fatalf("re-adding members: %v", err)
	}

	ids, _ := r.GetMemberIDs("room-1")
	if len(ids) != 2 {
		t.Errorf("got %d members after idempotent add, want 2", len(ids))
	}
}

func TestRoomRepository_RemoveMember(t *testing.T) {
	r := newRoomRepo(t)
	insertRoom(t, r, "room-1", "group")
	r.AddMember("room-1", "alice")
	r.AddMember("room-1", "bob")

	if err := r.RemoveMember("room-1", "alice"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	ids, _ := r.GetMemberIDs("room-1")
	if len(ids) != 1 || ids[0] != "bob" {
		t.Errorf("got members %v, want [bob]", ids)
	}
}

func TestRoomRepository_RemoveMember_NotFound(t *testing.T) {
	r := newRoomRepo(t)
	insertRoom(t, r, "room-1", "group")

	err := r.RemoveMember("room-1", "nobody")
	if err == nil {
		t.Error("expected error removing non-existent member")
	}
}

func TestRoomRepository_GetUserRooms(t *testing.T) {
	r := newRoomRepo(t)
	insertRoom(t, r, "room-a", "group")
	insertRoom(t, r, "room-b", "group")
	insertRoom(t, r, "room-c", "group")
	r.AddMember("room-a", "alice")
	r.AddMember("room-b", "alice")
	// room-c has no alice

	rooms, err := r.GetUserRooms("alice")
	if err != nil {
		t.Fatalf("GetUserRooms: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("got %d rooms, want 2", len(rooms))
	}
}
