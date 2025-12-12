package rediswr

import (
	"strings"

	"github.com/redis/go-redis/v9"
)

// New creates a new Redis client.
func New(cfg Config) redis.Cmdable {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:         strings.Split(cfg.Addrs, ","),
		Password:      cfg.Password,
		IsClusterMode: cfg.IsClusterMode,
	})

	return client
}
