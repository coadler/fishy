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
)

const (
	FishyTimeout      = 10 * time.Second
	GatherBaitTimeout = 6 * time.Hour
	ScoreGlobalKey    = "exp:global"
)
