package main

var (
	LocDensityKey = func(userID string) string { return "user:locationdensity:" + userID }
	RateLimitKey  = func(cmd string, userID string) string { return "ratelimit:" + cmd + ":" + userID }
)
