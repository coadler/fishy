package main

import (
	"time"
)

var (
	LocDensityKey = func(userID string) string { return "user:locationdensity:" + userID }
	LocationKey   = func(userID string) string { return "user:location:" + userID }
	InventoryKey  = func(userID string) string { return "user:inventory:" + userID }
	RateLimitKey  = func(cmd string, userID string) string { return "ratelimit:" + cmd + ":" + userID }
	ScoreGuildKey = func(guildID string) string { return "exp:guild:" + guildID }
	BlackListKey  = func(userID string) string { return "user:blacklist:" + userID }
	GatherBaitKey = func(userID string) string { return "user:gatherbait:" + userID }
	Morning1      = time.Date(0, 0, 0, 9, 0, 0, 0, time.UTC)
	Morning2      = time.Date(0, 0, 0, 15, 59, 59, 999, time.UTC)
	Night1        = time.Date(0, 0, 0, 16, 0, 0, 0, time.UTC)
	Night2        = time.Date(0, 0, 0, 8, 59, 59, 999, time.UTC)
	CurrentTime   = time.Now().UTC()
)

const (
	FishyTimeout      = 10 * time.Second
	GatherBaitTimeout = 6 * time.Hour
	ScoreGlobalKey    = "exp:global"
)
