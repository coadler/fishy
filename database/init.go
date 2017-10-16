package database

import (
	"time"

	"github.com/go-redis/redis"
)

var redisClient *redis.Client

// Init creates a redis client and attempts to ping the server.
func Init(url, password string, db int) error {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     url,
		Password: password,
		DB:       db,
	})

	// Prune stats timer
	p := time.Tick(1 * time.Minute)
	go func() {
		for {
			select {
			case <-p:
				go pruneStats()
			}
		}
	}()

	return redisClient.Ping().Err()
}
