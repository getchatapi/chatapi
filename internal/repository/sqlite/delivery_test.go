package sqlite_test

import (
	"testing"

	"github.com/getchatapi/chatapi/internal/repository/sqlite"
	"github.com/getchatapi/chatapi/internal/testutil"
)

func newDeliveryRepo(t *testing.T) *sqlite.SQLiteDeliveryRepository {
	t.Helper()
	db := testutil.NewTestDB(t)
	return sqlite.NewDeliveryRepository(db.DB)
}

func TestDeliveryRepository_QueueAndGet(t *testing.T) {
	r := newDeliveryRepo(t)

	if err := r.QueueUndelivered("user-1", "room-1", "msg-1", 1); err != nil {
		t.Fatalf("QueueUndelivered: %v", err)
	}
	if err := r.QueueUndelivered("user-1", "room-1", "msg-2", 2); err != nil {
		t.Fatalf("QueueUndelivered: %v", err)
	}

	pending, err := r.GetPendingUndelivered(3, 10)
	if err != nil {
		t.Fatalf("GetPendingUndelivered: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("got %d pending, want 2", len(pending))
	}
}

func TestDeliveryRepository_GetPendingUndelivered_Empty(t *testing.T) {
	r := newDeliveryRepo(t)

	pending, err := r.GetPendingUndelivered(3, 10)
	if err != nil {
		t.Fatalf("GetPendingUndelivered: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d pending, want 0", len(pending))
	}
}

func TestDeliveryRepository_MarkDelivered(t *testing.T) {
	r := newDeliveryRepo(t)

	r.QueueUndelivered("user-1", "room-1", "msg-1", 1)

	pending, _ := r.GetPendingUndelivered(3, 10)
	if len(pending) != 1 {
		t.Fatalf("setup: expected 1 pending, got %d", len(pending))
	}

	if err := r.MarkMessageDelivered(pending[0].ID); err != nil {
		t.Fatalf("MarkMessageDelivered: %v", err)
	}

	pending, _ = r.GetPendingUndelivered(3, 10)
	if len(pending) != 0 {
		t.Errorf("got %d pending after mark, want 0", len(pending))
	}
}

func TestDeliveryRepository_MaxAttempts(t *testing.T) {
	r := newDeliveryRepo(t)

	r.QueueUndelivered("user-1", "room-1", "msg-1", 1)

	// With maxAttempts=0, nothing should be returned (0 < 0 is false).
	pending, _ := r.GetPendingUndelivered(0, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 with maxAttempts=0, got %d", len(pending))
	}

	// With maxAttempts=1, the fresh entry (attempts=0) should appear.
	pending, _ = r.GetPendingUndelivered(1, 10)
	if len(pending) != 1 {
		t.Errorf("expected 1 with maxAttempts=1, got %d", len(pending))
	}
}

func TestDeliveryRepository_Limit(t *testing.T) {
	r := newDeliveryRepo(t)

	for i := 0; i < 5; i++ {
		r.QueueUndelivered("user-1", "room-1", "msg", i+1)
	}

	pending, _ := r.GetPendingUndelivered(3, 3)
	if len(pending) != 3 {
		t.Errorf("got %d with limit=3, want 3", len(pending))
	}
}
