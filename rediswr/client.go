package rediswr

import (
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rise-and-shine/pkg/meta"
)

// New creates a new Redis client.
func New(cfg Config) redis.Cmdable {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:         strings.Split(cfg.Addrs, ","),
		ClientName:    meta.ServiceName(),
		Username:      cfg.Username,
		Password:      cfg.Password,
		IsClusterMode: cfg.IsClusterMode,
	})

	return client
}
