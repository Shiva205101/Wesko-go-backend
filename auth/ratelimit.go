package auth

import (
	"context"
	"time"
)

type RateLimitRule struct {
	Limit  int
	Window time.Duration
}

type OTPRateLimitConfig struct {
	RequestIP     RateLimitRule
	RequestMobile RateLimitRule
	VerifyIP      RateLimitRule
	VerifyMobile  RateLimitRule
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, rule RateLimitRule) (bool, error)
}
