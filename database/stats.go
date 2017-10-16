package database

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

func pruneStats() {
	hour := fmt.Sprintf("%v", time.Now().Add(-1*time.Hour).Unix())
	day := fmt.Sprintf("%v", time.Now().Add(-24*time.Hour).Unix())
	redisClient.ZRemRangeByScore(hourlyCmdTrack("fish"), "0", hour)
	redisClient.ZRemRangeByScore(dailyCmdTrack("fish"), "0", day)
}

// CmdStats is the main function for tracking commands.
func CmdStats(cmd, uID string) {
	switch cmd {
	case "fish":
		redisClient.Incr(totalCmdTrack("fish"))
		now := float64(time.Now().Unix())
		redisClient.ZAdd(hourlyCmdTrack("fish"), redis.Z{Score: now, Member: uID})
		redisClient.ZAdd(dailyCmdTrack("fish"), redis.Z{Score: now, Member: uID})
	}
}
