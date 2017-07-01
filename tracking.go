package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

func init() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     Config.Redis.URL,
		Password: Config.Redis.Password,
		DB:       Config.Redis.DB,
	})
	if err := redisClient.Ping().Err(); err != nil {
		panic(err)
	}

	p := time.Tick(1 * time.Minute)

	go func() {
		for {
			select {
			case <-p:
				go pruneStats()
			}
		}

	}()
}

func pruneStats() {
	hour := fmt.Sprintf("%v", time.Now().Add(-1*time.Hour).Unix())
	day := fmt.Sprintf("%v", time.Now().Add(-24*time.Hour).Unix())
	redisClient.ZRemRangeByScore(HourlyCmdTrack("fish"), "0", hour)
	redisClient.ZRemRangeByScore(DailyCmdTrack("fish"), "0", day)
}

// CmdStats is the main function for tracking commands
func CmdStats(cmd, uID string) {
	switch cmd {
	case "fish":
		go incrFish(uID)
	}
}

func incrFish(uID string) {
	redisClient.Incr(TotalCmdTrack("fish"))
	now := float64(time.Now().Unix())
	redisClient.ZAdd(HourlyCmdTrack("fish"), redis.Z{Score: now, Member: uID})
	redisClient.ZAdd(DailyCmdTrack("fish"), redis.Z{Score: now, Member: uID})
}
