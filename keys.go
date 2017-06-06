package main

var (
	LocDensityKey = func(userID string) string { return "user:locationdensity:" + userID }
	LocationKey   = func(userID string) string { return "user:location:" + userID }
	RateLimitKey  = func(cmd string, userID string) string { return "ratelimit:" + cmd + ":" + userID }
)
