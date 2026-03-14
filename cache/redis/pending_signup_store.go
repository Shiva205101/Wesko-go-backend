package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"vesko/auth"

	goredis "github.com/redis/go-redis/v9"
)

type PendingSignupStore struct {
	client *goredis.Client
	prefix string
}

func NewPendingSignupStore(client *goredis.Client) *PendingSignupStore {
	return &PendingSignupStore{
		client: client,
		prefix: "auth:pending_signup:",
	}
}

func (s *PendingSignupStore) Save(ctx context.Context, signup auth.PendingSignup) error {
	payload, err := json.Marshal(signup)
	if err != nil {
		return err
	}

	ttl := time.Until(signup.ExpiresAt)
	if ttl <= 0 {
		return auth.ErrPendingSignupExpired
	}

	// Store the pending payload plus reverse indices so signup conflicts can be detected by username, email, or mobile.
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.signupKey(signup.Mobile), payload, ttl)
	pipe.Set(ctx, s.usernameKey(signup.Username), signup.Mobile, ttl)
	pipe.Set(ctx, s.emailKey(signup.Email), signup.Mobile, ttl)
	pipe.Set(ctx, s.mobileIndexKey(signup.Mobile), signup.Mobile, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *PendingSignupStore) GetByMobile(ctx context.Context, mobile string) (auth.PendingSignup, error) {
	payload, err := s.client.Get(ctx, s.signupKey(mobile)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return auth.PendingSignup{}, auth.ErrCacheMiss
		}
		return auth.PendingSignup{}, err
	}

	var signup auth.PendingSignup
	if err := json.Unmarshal(payload, &signup); err != nil {
		return auth.PendingSignup{}, err
	}

	return signup, nil
}

func (s *PendingSignupStore) DeleteByMobile(ctx context.Context, mobile string) error {
	signup, err := s.GetByMobile(ctx, mobile)
	if err != nil {
		if errors.Is(err, auth.ErrCacheMiss) {
			return nil
		}
		return err
	}

	pipe := s.client.TxPipeline()
	// Remove both the payload and its identity indices together to keep signup reservation state consistent.
	pipe.Del(ctx, s.signupKey(mobile))
	pipe.Del(ctx, s.usernameKey(signup.Username))
	pipe.Del(ctx, s.emailKey(signup.Email))
	pipe.Del(ctx, s.mobileIndexKey(signup.Mobile))
	_, err = pipe.Exec(ctx)
	return err
}

func (s *PendingSignupStore) GetMobileByUsername(ctx context.Context, username string) (string, error) {
	return s.getIndex(ctx, s.usernameKey(username))
}

func (s *PendingSignupStore) GetMobileByEmail(ctx context.Context, email string) (string, error) {
	return s.getIndex(ctx, s.emailKey(email))
}

func (s *PendingSignupStore) GetMobileByMobile(ctx context.Context, mobile string) (string, error) {
	return s.getIndex(ctx, s.mobileIndexKey(mobile))
}

func (s *PendingSignupStore) getIndex(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", auth.ErrCacheMiss
		}
		return "", err
	}

	return value, nil
}

func (s *PendingSignupStore) signupKey(mobile string) string {
	return fmt.Sprintf("%smobile:%s", s.prefix, mobile)
}

func (s *PendingSignupStore) usernameKey(username string) string {
	return fmt.Sprintf("%susername:%s", s.prefix, username)
}

func (s *PendingSignupStore) emailKey(email string) string {
	return fmt.Sprintf("%semail:%s", s.prefix, email)
}

func (s *PendingSignupStore) mobileIndexKey(mobile string) string {
	return fmt.Sprintf("%sindex_mobile:%s", s.prefix, mobile)
}
