package botutils

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var Ctx = context.Background()

func NewRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func SetValue(client *redis.Client, key string, value interface{}) error {
	return client.Set(Ctx, key, value, 0).Err()
}

func GetValue(client *redis.Client, key string) (string, error) {
	return client.Get(Ctx, key).Result()
}
