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

type UserCache struct {
	client *goredis.Client
	ttl    time.Duration
	prefix string
}

func NewUserCache(client *goredis.Client, ttl time.Duration) *UserCache {
	return &UserCache{
		client: client,
		ttl:    ttl,
		prefix: "auth:user:",
	}
}

func (c *UserCache) GetUser(ctx context.Context, username string) (auth.User, error) {
	payload, err := c.client.Get(ctx, c.cacheKey(username)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return auth.User{}, auth.ErrCacheMiss
		}
		return auth.User{}, err
	}

	var user auth.User
	if err := json.Unmarshal(payload, &user); err != nil {
		return auth.User{}, err
	}

	return user, nil
}

func (c *UserCache) SetUser(ctx context.Context, user auth.User) error {
	payload, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, c.cacheKey(user.Username), payload, c.ttl).Err()
}

func (c *UserCache) DeleteUser(ctx context.Context, username string) error {
	return c.client.Del(ctx, c.cacheKey(username)).Err()
}

func (c *UserCache) cacheKey(username string) string {
	return fmt.Sprintf("%s%s", c.prefix, username)
}
