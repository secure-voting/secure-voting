package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, addr, password string, useTLS bool, caPath string, serverName string) (*redis.Client, error) {
	opt := &redis.Options{
		Addr:        addr,
		Password:    password,
		DB:          0,
		DialTimeout: 3 * time.Second,
	}

	if useTLS {
		if caPath == "" {
			return nil, fmt.Errorf("redis tls enabled but ca path is empty")
		}

		if serverName == "" {
			serverName = "cache"
		}

		caPEM, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read redis ca: %w", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("append redis ca failed")
		}

		opt.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
			ServerName: serverName,
		}
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
