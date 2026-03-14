package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"vesko/auth"

	goredis "github.com/redis/go-redis/v9"
)

type RefreshStore struct {
	client *goredis.Client
	prefix string
}

func NewRefreshStore(client *goredis.Client) *RefreshStore {
	return &RefreshStore{
		client: client,
		prefix: "auth:refresh:",
	}
}

func (s *RefreshStore) Save(ctx context.Context, rawToken string, session auth.RefreshSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}

	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return auth.ErrTokenExpired
	}

	return s.client.Set(ctx, s.key(rawToken), payload, ttl).Err()
}

func (s *RefreshStore) Get(ctx context.Context, rawToken string) (auth.RefreshSession, error) {
	payload, err := s.client.Get(ctx, s.key(rawToken)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return auth.RefreshSession{}, auth.ErrCacheMiss
		}
		return auth.RefreshSession{}, err
	}

	var session auth.RefreshSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return auth.RefreshSession{}, err
	}

	return session, nil
}

func (s *RefreshStore) Delete(ctx context.Context, rawToken string) error {
	return s.client.Del(ctx, s.key(rawToken)).Err()
}

func (s *RefreshStore) key(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return fmt.Sprintf("%s%s", s.prefix, hex.EncodeToString(sum[:]))
}
