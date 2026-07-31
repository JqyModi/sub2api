package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type desktopAuthStore struct {
	client *redis.Client
}

func NewDesktopAuthStore(client *redis.Client) service.DesktopAuthStore {
	return &desktopAuthStore{client: client}
}

func (s *desktopAuthStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", service.ErrDesktopAuthStoreNotFound
	}
	return value, err
}

func (s *desktopAuthStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *desktopAuthStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
