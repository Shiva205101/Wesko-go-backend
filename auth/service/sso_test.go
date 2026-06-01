package service

import (
	"context"
	"testing"
	"vesko/auth"
)

func TestCompleteProfile(t *testing.T) {
	t.Parallel()

	service, repo, _, _, _ := newTestService(t)

	// Create a partial user
	user, err := repo.CreateSSOUser(context.Background(), auth.User{
		Username:          "google_user",
		Email:             "google@example.com",
		Role:              auth.RoleCustomer,
		IsProfileComplete: false,
	}, auth.SSOAccount{
		Provider:   "google",
		ProviderID: "google-id",
		Email:      "google@example.com",
	})
	if err != nil {
		t.Fatalf("create sso user: %v", err)
	}

	// Complete profile
	updatedUser, tokens, err := service.CompleteProfile(context.Background(), user.ID, "new_username", "9642560235")
	if err != nil {
		t.Fatalf("complete profile: %v", err)
	}

	if updatedUser.Username != "new_username" {
		t.Fatalf("expected username new_username, got %s", updatedUser.Username)
	}
	if updatedUser.Mobile != "+919642560235" {
		t.Fatalf("expected mobile +919642560235, got %s", updatedUser.Mobile)
	}
	if !updatedUser.IsProfileComplete {
		t.Fatalf("expected profile to be complete")
	}
	if tokens.AccessToken == "" {
		t.Fatalf("expected access token")
	}

	// Verify in repo
	stored, err := repo.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if !stored.IsProfileComplete {
		t.Fatalf("stored user profile should be complete")
	}
}

func TestCompleteProfileFailsForCompleteUser(t *testing.T) {
	t.Parallel()

	service, repo, _, _, _ := newTestService(t)

	// Create a complete user
	user, err := repo.RegisterUser(context.Background(), auth.User{
		Username:          "john",
		Email:             "john@example.com",
		Mobile:            "+919642560235",
		IsProfileComplete: true,
	}, "hash")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	_, _, err = service.CompleteProfile(context.Background(), user.ID, "new", "9999999999")
	if err == nil {
		t.Fatalf("expected error for complete user")
	}
}
