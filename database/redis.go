package database

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(
	ctx context.Context,
	addr string,
	username string,
	password string,
	useTLS bool,
) (*redis.Client, error){
	if addr == "" {
		return nil, errors.New("REDIS_ADDR is not configured")
	}

	opts := &redis.Options{
		Addr: addr,
		Username:     username,
		Password:     password,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	if useTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}