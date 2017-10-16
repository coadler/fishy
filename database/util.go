package database

import (
	"encoding/json"
	"time"
)

// keyExists returns true if the provided `key` exists in Redis.
func keyExists(key string) bool {
	return redisClient.Exists(key).Val() == 1
}

// marshalAndSet takes a encoding/json marshalable interface{} and sets the `key` in Redis to it's JSON-encoded value
// with the provided expiration value.
func marshalAndSet(data interface{}, key string, expiration time.Duration) error {
	set, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = redisClient.Set(key, set, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}
