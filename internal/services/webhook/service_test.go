package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getchatapi/chatapi/internal/services/webhook"
)

func TestPost(t *testing.T) {
	t.Run("sends JSON body and returns response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		}))
		defer srv.Close()

		svc := webhook.NewService()
		body, err := svc.Post(srv.URL, "", map[string]string{"hello": "world"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(body) != `{"ok":true}` {
			t.Errorf("got body %q, want %q", body, `{"ok":true}`)
		}
	})

	t.Run("signs request when secret is set", func(t *testing.T) {
		const secret = "mysecret"
		payload := map[string]string{"type": "bot.context"}

		var receivedSig string
		var receivedBody []byte

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedSig = r.Header.Get("X-ChatAPI-Signature")
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		svc := webhook.NewService()
		_, err := svc.Post(srv.URL, secret, payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the HMAC signature.
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(receivedBody)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if receivedSig != want {
			t.Errorf("signature mismatch: got %q, want %q", receivedSig, want)
		}
	})

	t.Run("returns error on non-2xx response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		svc := webhook.NewService()
		_, err := svc.Post(srv.URL, "", map[string]string{})
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})

	t.Run("returns error when server unreachable", func(t *testing.T) {
		svc := webhook.NewService()
		_, err := svc.Post("http://127.0.0.1:1", "", map[string]string{})
		if err == nil {
			t.Fatal("expected error for unreachable server")
		}
	})
}

func TestNotifyOfflineUser(t *testing.T) {
	t.Run("sends correct payload", func(t *testing.T) {
		var got webhook.OfflineMessagePayload

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&got)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		svc := webhook.NewService()
		msg := webhook.MessageInfo{
			MessageID: "msg-1",
			SenderID:  "user-a",
			Content:   "hello",
			Seq:       1,
			CreatedAt: time.Now().UTC(),
		}
		svc.NotifyOfflineUser(srv.URL, "", "room-1", "user-b", "", msg)

		if got.Type != "message.offline" {
			t.Errorf("got type %q, want %q", got.Type, "message.offline")
		}
		if got.RoomID != "room-1" {
			t.Errorf("got room_id %q, want %q", got.RoomID, "room-1")
		}
		if got.RecipientID != "user-b" {
			t.Errorf("got recipient_id %q, want %q", got.RecipientID, "user-b")
		}
		if got.Message.Content != "hello" {
			t.Errorf("got message content %q, want %q", got.Message.Content, "hello")
		}
	})

	t.Run("no-ops when URL is empty", func(t *testing.T) {
		// Should not panic or error.
		svc := webhook.NewService()
		svc.NotifyOfflineUser("", "", "room-1", "user-b", "", webhook.MessageInfo{})
	})

	t.Run("signs request when secret is set", func(t *testing.T) {
		const secret = "webhook-secret"
		var receivedSig string
		var receivedBody []byte

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedSig = r.Header.Get("X-ChatAPI-Signature")
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		svc := webhook.NewService()
		svc.NotifyOfflineUser(srv.URL, secret, "room-1", "user-b", "", webhook.MessageInfo{
			MessageID: "m1",
			SenderID:  "user-a",
			Content:   "ping",
			Seq:       1,
			CreatedAt: time.Now().UTC(),
		})

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(receivedBody)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if receivedSig != want {
			t.Errorf("signature mismatch: got %q, want %q", receivedSig, want)
		}
	})
}
