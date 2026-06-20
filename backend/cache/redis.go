package cache

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"

	"api-mocker/config"
)

func Connect(cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr(),
		DB:   0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	fmt.Println("Connected to Redis")
	return rdb
}
