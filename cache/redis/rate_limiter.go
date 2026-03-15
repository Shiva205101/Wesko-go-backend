package redis

import (
	"context"
	"fmt"

	"vesko/auth"

	goredis "github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *goredis.Client
	prefix string
}

func NewRateLimiter(client *goredis.Client) *RateLimiter {
	return &RateLimiter{
		client: client,
		prefix: "auth:rate_limit:",
	}
}

func (l *RateLimiter) Allow(ctx context.Context, key string, rule auth.RateLimitRule) (bool, error) {
	if rule.Limit <= 0 || rule.Window <= 0 {
		return true, nil
	}

	redisKey := fmt.Sprintf("%s%s", l.prefix, key)
	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := l.client.Expire(ctx, redisKey, rule.Window).Err(); err != nil {
			return false, err
		}
	}

	return count <= int64(rule.Limit), nil
}
