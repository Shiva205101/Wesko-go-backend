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

type OTPStateStore struct {
	client *goredis.Client
	prefix string
}

func NewOTPStateStore(client *goredis.Client) *OTPStateStore {
	return &OTPStateStore{
		client: client,
		prefix: "auth:otp_state:",
	}
}

func (s *OTPStateStore) Save(ctx context.Context, key string, state auth.OTPRequestState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}

	ttl := time.Until(state.ExpiresAt)
	if ttl <= 0 {
		return auth.ErrTokenExpired
	}

	return s.client.Set(ctx, s.key(key), payload, ttl).Err()
}

func (s *OTPStateStore) Get(ctx context.Context, key string) (auth.OTPRequestState, error) {
	payload, err := s.client.Get(ctx, s.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return auth.OTPRequestState{}, auth.ErrCacheMiss
		}
		return auth.OTPRequestState{}, err
	}

	var state auth.OTPRequestState
	if err := json.Unmarshal(payload, &state); err != nil {
		return auth.OTPRequestState{}, err
	}

	return state, nil
}

func (s *OTPStateStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

func (s *OTPStateStore) key(key string) string {
	return fmt.Sprintf("%s%s", s.prefix, key)
}
