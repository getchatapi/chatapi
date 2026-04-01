package delivery_test

import (
	"context"
	"testing"

	"github.com/hastenr/chatapi/internal/models"
	"github.com/hastenr/chatapi/internal/services/chatroom"
	"github.com/hastenr/chatapi/internal/services/delivery"
	"github.com/hastenr/chatapi/internal/services/message"
	"github.com/hastenr/chatapi/internal/services/notification"
	"github.com/hastenr/chatapi/internal/services/realtime"
	"github.com/hastenr/chatapi/internal/services/tenant"
	"github.com/hastenr/chatapi/internal/services/webhook"
	"github.com/hastenr/chatapi/internal/testutil"
)

type deliveryScenario struct {
	tenantID    string
	roomID      string
	deliverySvc *delivery.Service
	messageSvc  *message.Service
	realtimeSvc *realtime.Service
	notifSvc    *notification.Service
}

func newDeliveryScenario(t *testing.T) *deliveryScenario {
	t.Helper()
	db := testutil.NewTestDB(t)

	tenantSvc := tenant.NewService(db.DB)
	ten, err := tenantSvc.CreateTenant("test")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	chatroomSvc := chatroom.NewService(db.DB)
	room, err := chatroomSvc.CreateRoom(ten.TenantID, &models.CreateRoomRequest{
		Type:    "group",
		Name:    "general",
		Members: []string{"user1", "user2", "user3"},
	})
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	realtimeSvc := realtime.NewService(db.DB, 5)
	t.Cleanup(func() { realtimeSvc.Shutdown(context.Background()) })

	webhookSvc := webhook.NewService()
	deliverySvc := delivery.NewService(db.DB, realtimeSvc, chatroomSvc, tenantSvc, webhookSvc)
	messageSvc := message.NewService(db.DB)
	notifSvc := notification.NewService(db.DB)

	return &deliveryScenario{
		tenantID:    ten.TenantID,
		roomID:      room.RoomID,
		deliverySvc: deliverySvc,
		messageSvc:  messageSvc,
		realtimeSvc: realtimeSvc,
		notifSvc:    notifSvc,
	}
}

// --- HandleNewMessage ---

func TestHandleNewMessage_QueuesForOfflineUsers(t *testing.T) {
	s := newDeliveryScenario(t)

	// Send a message from user1 — user2 and user3 are offline
	msg, err := s.messageSvc.SendMessage(s.tenantID, s.roomID, "user1", &models.CreateMessageRequest{
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	s.deliverySvc.HandleNewMessage(s.tenantID, s.roomID, msg)

	// Both offline recipients should have an undelivered entry
	undelivered, err := s.messageSvc.GetUndeliveredMessages(s.tenantID, "user2", 50)
	if err != nil {
		t.Fatalf("GetUndeliveredMessages(user2): %v", err)
	}
	if len(undelivered) != 1 {
		t.Errorf("user2 undelivered count = %d, want 1", len(undelivered))
	}

	undelivered, err = s.messageSvc.GetUndeliveredMessages(s.tenantID, "user3", 50)
	if err != nil {
		t.Fatalf("GetUndeliveredMessages(user3): %v", err)
	}
	if len(undelivered) != 1 {
		t.Errorf("user3 undelivered count = %d, want 1", len(undelivered))
	}
}

func TestHandleNewMessage_SenderNotQueued(t *testing.T) {
	s := newDeliveryScenario(t)

	msg, err := s.messageSvc.SendMessage(s.tenantID, s.roomID, "user1", &models.CreateMessageRequest{
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	s.deliverySvc.HandleNewMessage(s.tenantID, s.roomID, msg)

	// Sender must not appear in the undelivered queue
	undelivered, err := s.messageSvc.GetUndeliveredMessages(s.tenantID, "user1", 50)
	if err != nil {
		t.Fatalf("GetUndeliveredMessages(user1): %v", err)
	}
	if len(undelivered) != 0 {
		t.Errorf("sender has %d undelivered entries, want 0", len(undelivered))
	}
}

// --- ProcessUndeliveredMessages ---

func TestProcessUndeliveredMessages_IncreasesAttempts(t *testing.T) {
	s := newDeliveryScenario(t)

	msg, _ := s.messageSvc.SendMessage(s.tenantID, s.roomID, "user1", &models.CreateMessageRequest{Content: "hi"})
	s.deliverySvc.HandleNewMessage(s.tenantID, s.roomID, msg)

	// First pass — all offline, attempts go from 0 to 1
	if err := s.deliverySvc.ProcessUndeliveredMessages(s.tenantID, 50); err != nil {
		t.Fatalf("ProcessUndeliveredMessages: %v", err)
	}

	undelivered, _ := s.messageSvc.GetUndeliveredMessages(s.tenantID, "user2", 50)
	if len(undelivered) == 0 {
		t.Fatal("expected undelivered entry after processing")
	}
	if undelivered[0].Attempts != 1 {
		t.Errorf("attempts = %d, want 1", undelivered[0].Attempts)
	}
}

func TestProcessUndeliveredMessages_DeliveredCounters(t *testing.T) {
	s := newDeliveryScenario(t)

	msg, _ := s.messageSvc.SendMessage(s.tenantID, s.roomID, "user1", &models.CreateMessageRequest{Content: "hi"})
	s.deliverySvc.HandleNewMessage(s.tenantID, s.roomID, msg)

	before := s.deliverySvc.DeliveryAttempts()
	s.deliverySvc.ProcessUndeliveredMessages(s.tenantID, 50)
	after := s.deliverySvc.DeliveryAttempts()

	// user2 + user3 are both offline → 2 attempts
	if after-before != 2 {
		t.Errorf("delivery attempts delta = %d, want 2", after-before)
	}
}

// --- DeliverNow (notifications) ---

func TestDeliverNow_DoesNotPanic(t *testing.T) {
	s := newDeliveryScenario(t)

	notif, err := s.notifSvc.CreateNotification(s.tenantID, &models.CreateNotificationRequest{
		Topic:   "test.topic",
		Payload: map[string]interface{}{"key": "value"},
		Targets: models.NotificationTargets{UserIDs: []string{"user1"}},
	})
	if err != nil {
		t.Fatalf("CreateNotification: %v", err)
	}

	// Should not panic even when user is offline
	s.deliverySvc.DeliverNow(notif)
}

// --- DeliveryAttempts / DeliveryFailures counters ---

func TestDeliveryCounters_InitiallyZero(t *testing.T) {
	s := newDeliveryScenario(t)

	if got := s.deliverySvc.DeliveryAttempts(); got != 0 {
		t.Errorf("DeliveryAttempts = %d, want 0", got)
	}
	if got := s.deliverySvc.DeliveryFailures(); got != 0 {
		t.Errorf("DeliveryFailures = %d, want 0", got)
	}
}

func TestDeliveryFailures_IncrementsWhenOffline(t *testing.T) {
	s := newDeliveryScenario(t)

	msg, _ := s.messageSvc.SendMessage(s.tenantID, s.roomID, "user1", &models.CreateMessageRequest{Content: "hi"})
	s.deliverySvc.HandleNewMessage(s.tenantID, s.roomID, msg)
	s.deliverySvc.ProcessUndeliveredMessages(s.tenantID, 50)

	if got := s.deliverySvc.DeliveryFailures(); got == 0 {
		t.Error("DeliveryFailures = 0 after processing offline users, want > 0")
	}
}
