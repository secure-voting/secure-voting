package db

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, addr, password string) (*redis.Client, error) {
	opt := &redis.Options{
		Addr:        addr,
		Password:    password,
		DB:          0,
		DialTimeout: 3 * time.Second,
	}
	rdb := redis.NewClient(opt)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return rdb, nil
}
