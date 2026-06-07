package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

func Connect(ctx context.Context, url string) (*Redis, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &Redis{Client: redis.NewClient(opt)}, nil
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.Client.Close()
}
