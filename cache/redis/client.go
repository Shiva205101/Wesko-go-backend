package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Enabled      bool
	Addr         string
	Username     string
	Password     string
	DB           int
	CacheTTL     time.Duration
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
}

func (cfg Config) NewClient() *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	})
}

func Ping(ctx context.Context, client *goredis.Client) error {
	return client.Ping(ctx).Err()
}
