package database

import "github.com/go-redis/redis"

var redisClient *redis.Client

// Init creates a redis client and attempts to ping the server.
func Init(url, password string, db int) error {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     url,
		Password: password,
		DB:       db,
	})
	return redisClient.Ping().Err()
}
