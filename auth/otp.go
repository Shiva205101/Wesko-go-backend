package auth

import (
	"context"
	"time"
)

type OTPProvider interface {
	SendVerification(ctx context.Context, mobile string) error
	CheckVerification(ctx context.Context, mobile string, code string) (bool, error)
}

type PendingSignup struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Email        string    `json:"email"`
	Mobile       string    `json:"mobile"`
	ResendCount  int       `json:"resend_count"`
	LastSentAt   time.Time `json:"last_sent_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type PendingSignupStore interface {
	Save(ctx context.Context, signup PendingSignup) error
	GetByMobile(ctx context.Context, mobile string) (PendingSignup, error)
	DeleteByMobile(ctx context.Context, mobile string) error
	GetMobileByUsername(ctx context.Context, username string) (string, error)
	GetMobileByEmail(ctx context.Context, email string) (string, error)
	GetMobileByMobile(ctx context.Context, mobile string) (string, error)
}

type OTPRequestState struct {
	Mobile      string    `json:"mobile"`
	ResendCount int       `json:"resend_count"`
	LastSentAt  time.Time `json:"last_sent_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type OTPRequestStateStore interface {
	Save(ctx context.Context, key string, state OTPRequestState) error
	Get(ctx context.Context, key string) (OTPRequestState, error)
	Delete(ctx context.Context, key string) error
}

type OTPConfig struct {
	PendingSignupTTL time.Duration
	ResendCooldown   time.Duration
	MaxResends       int
}
