package main

var (
	LocDensityKey  = func(userID string) string { return "user:locationdensity:" + userID }
	LocationKey    = func(userID string) string { return "user:location:" + userID }
	InventoryKey   = func(userID string) string { return "user:inventory:" + userID }
	RateLimitKey   = func(cmd string, userID string) string { return "ratelimit:" + cmd + ":" + userID }
	ScoreGuildKey  = func(guildID string) string { return "exp:guild:" + guildID }
	ScoreGlobalKey = "exp:global"
)
