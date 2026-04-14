package broker_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getchatapi/chatapi/internal/broker"
)

// wait polls fn until it returns true or the timeout elapses.
func wait(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestLocalBroker_Dispatch(t *testing.T) {
	var mu sync.Mutex
	var got []string

	b := broker.NewLocalBroker(func(roomID string, _ []byte) {
		mu.Lock()
		got = append(got, roomID)
		mu.Unlock()
	})
	defer b.Close()

	b.Broadcast("room-1", []byte(`{}`))
	b.Broadcast("room-2", []byte(`{}`))
	b.Broadcast("room-1", []byte(`{}`))

	wait(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 3
	})

	mu.Lock()
	defer mu.Unlock()
	if got[0] != "room-1" || got[1] != "room-2" || got[2] != "room-1" {
		t.Errorf("dispatch order: got %v", got)
	}
}

func TestLocalBroker_PayloadDelivered(t *testing.T) {
	done := make(chan []byte, 1)

	b := broker.NewLocalBroker(func(_ string, payload []byte) {
		done <- payload
	})
	defer b.Close()

	b.Broadcast("room-1", []byte(`{"type":"msg"}`))

	select {
	case p := <-done:
		if string(p) != `{"type":"msg"}` {
			t.Errorf("payload: got %q", p)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for payload")
	}
}

func TestLocalBroker_DropsWhenFull(t *testing.T) {
	// Block the deliver function so the channel fills up.
	block := make(chan struct{})
	var delivered atomic.Int64

	b := broker.NewLocalBroker(func(_ string, _ []byte) {
		<-block
		delivered.Add(1)
	})
	defer func() {
		close(block)
		b.Close()
	}()

	// Send more than the channel capacity (1000) to guarantee drops.
	for i := 0; i < 1100; i++ {
		b.Broadcast("room-1", []byte(`{}`))
	}

	if b.DroppedCount() == 0 {
		t.Error("expected at least one dropped message")
	}
	if b.DroppedCount() > 1100 {
		t.Errorf("dropped count %d exceeds total sent", b.DroppedCount())
	}
}

func TestLocalBroker_DroppedCount_ZeroInitially(t *testing.T) {
	b := broker.NewLocalBroker(func(_ string, _ []byte) {})
	defer b.Close()

	if b.DroppedCount() != 0 {
		t.Errorf("initial dropped count: got %d, want 0", b.DroppedCount())
	}
}

func TestLocalBroker_Close(t *testing.T) {
	b := broker.NewLocalBroker(func(_ string, _ []byte) {})
	// Close should not panic or block.
	b.Close()
}

func TestLocalBroker_MultipleRooms(t *testing.T) {
	counts := make(map[string]int)
	var mu sync.Mutex

	b := broker.NewLocalBroker(func(roomID string, _ []byte) {
		mu.Lock()
		counts[roomID]++
		mu.Unlock()
	})
	defer b.Close()

	for i := 0; i < 5; i++ {
		b.Broadcast("room-a", []byte(`{}`))
		b.Broadcast("room-b", []byte(`{}`))
	}

	wait(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return counts["room-a"] == 5 && counts["room-b"] == 5
	})
}
