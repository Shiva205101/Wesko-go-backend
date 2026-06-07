package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"vesko/auth"
)

func TestRefreshStoreExpiresSessions(t *testing.T) {
	store := NewRefreshStore()
	ctx := context.Background()

	if err := store.Save(ctx, "token", auth.RefreshSession{
		TokenID:   "id",
		UserID:    1,
		Username:  "user",
		ExpiresAt: time.Now().UTC().Add(-time.Second),
	}); !errors.Is(err, auth.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}

	if err := store.Save(ctx, "token", auth.RefreshSession{
		TokenID:   "id",
		UserID:    1,
		Username:  "user",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatalf("save refresh session: %v", err)
	}

	session, err := store.Get(ctx, "token")
	if err != nil {
		t.Fatalf("get refresh session: %v", err)
	}
	if session.Username != "user" {
		t.Fatalf("expected username user, got %q", session.Username)
	}

	if err := store.Delete(ctx, "token"); err != nil {
		t.Fatalf("delete refresh session: %v", err)
	}
	if _, err := store.Get(ctx, "token"); !errors.Is(err, auth.ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestPendingSignupStoreMaintainsIndexes(t *testing.T) {
	store := NewPendingSignupStore()
	ctx := context.Background()
	signup := auth.PendingSignup{
		Username:  "newuser",
		Email:     "new@example.com",
		Mobile:    "+919999999999",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	if err := store.Save(ctx, signup); err != nil {
		t.Fatalf("save pending signup: %v", err)
	}

	for name, fn := range map[string]func(context.Context, string) (string, error){
		"username": store.GetMobileByUsername,
		"email":    store.GetMobileByEmail,
		"mobile":   store.GetMobileByMobile,
	} {
		mobile, err := fn(ctx, map[string]string{
			"username": signup.Username,
			"email":    signup.Email,
			"mobile":   signup.Mobile,
		}[name])
		if err != nil {
			t.Fatalf("get index by %s: %v", name, err)
		}
		if mobile != signup.Mobile {
			t.Fatalf("expected mobile %q from %s index, got %q", signup.Mobile, name, mobile)
		}
	}

	if err := store.DeleteByMobile(ctx, signup.Mobile); err != nil {
		t.Fatalf("delete pending signup: %v", err)
	}
	if _, err := store.GetMobileByUsername(ctx, signup.Username); !errors.Is(err, auth.ErrCacheMiss) {
		t.Fatalf("expected username index miss after delete, got %v", err)
	}
}

func TestRateLimiterAllowsWithinWindow(t *testing.T) {
	limiter := NewRateLimiter()
	ctx := context.Background()
	rule := auth.RateLimitRule{Limit: 2, Window: time.Hour}

	for i := 0; i < 2; i++ {
		allowed, err := limiter.Allow(ctx, "otp:ip:127.0.0.1", rule)
		if err != nil {
			t.Fatalf("allow rate limit: %v", err)
		}
		if !allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	allowed, err := limiter.Allow(ctx, "otp:ip:127.0.0.1", rule)
	if err != nil {
		t.Fatalf("allow rate limit: %v", err)
	}
	if allowed {
		t.Fatal("expected third request to be blocked")
	}
}
