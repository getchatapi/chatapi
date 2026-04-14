package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/getchatapi/chatapi/internal/auth"
)

const testSecret = "test-secret-key"

func signToken(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func TestValidateJWT(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		token := signToken(t, jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		}, testSecret)

		userID, err := auth.ValidateJWT(testSecret, token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if userID != "user-123" {
			t.Errorf("got userID %q, want %q", userID, "user-123")
		}
	})

	t.Run("empty secret", func(t *testing.T) {
		_, err := auth.ValidateJWT("", "anytoken")
		if err == nil {
			t.Fatal("expected error for empty secret")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		token := signToken(t, jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		}, testSecret)

		_, err := auth.ValidateJWT("wrong-secret", token)
		if err == nil {
			t.Fatal("expected error for wrong secret")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token := signToken(t, jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(-1 * time.Minute).Unix(),
		}, testSecret)

		_, err := auth.ValidateJWT(testSecret, token)
		if err == nil {
			t.Fatal("expected error for expired token")
		}
	})

	t.Run("missing sub claim", func(t *testing.T) {
		token := signToken(t, jwt.MapClaims{
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		}, testSecret)

		_, err := auth.ValidateJWT(testSecret, token)
		if err == nil {
			t.Fatal("expected error for missing sub claim")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := auth.ValidateJWT(testSecret, "not.a.token")
		if err == nil {
			t.Fatal("expected error for malformed token")
		}
	})

	t.Run("wrong signing method rejected", func(t *testing.T) {
		// "none" alg token — should be rejected since we only accept HS256.
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		token, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)

		_, err := auth.ValidateJWT(testSecret, token)
		if err == nil {
			t.Fatal("expected error for 'none' alg token")
		}
	})
}

func TestUserIDContext(t *testing.T) {
	ctx := context.Background()

	// No user ID in fresh context.
	_, ok := auth.UserIDFromContext(ctx)
	if ok {
		t.Fatal("expected no user ID in empty context")
	}

	// Store and retrieve.
	ctx = auth.WithUserID(ctx, "user-abc")
	uid, ok := auth.UserIDFromContext(ctx)
	if !ok {
		t.Fatal("expected user ID in context")
	}
	if uid != "user-abc" {
		t.Errorf("got %q, want %q", uid, "user-abc")
	}

	// Empty string should not be ok.
	ctx2 := auth.WithUserID(context.Background(), "")
	_, ok = auth.UserIDFromContext(ctx2)
	if ok {
		t.Fatal("expected false for empty user ID")
	}
}

