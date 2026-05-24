package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var Ctx = context.Background()

func InitRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	password := os.Getenv("REDIS_PASSWORD")

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       0,        // use default DB
	})

	_, err := RedisClient.Ping(Ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis at %s: %v\n", addr, err)
		os.Exit(1)
	}
	fmt.Printf("Connected to Redis at %s\n", addr)
}
