package database

import (
	"fmt"
	"time"
)

var (
	baitGatherKey       = func(userID string) string { return "bait:gathering:" + userID }
	baitInvKey          = func(userID string) string { return "bait:inventory:" + userID }
	baitTierKey         = func(userID string) string { return "bait:tier:" + userID }
	blackListKey        = func(userID string) string { return "user:blacklist:" + userID }
	commandRateLimitKey = func(cmd, userID string) string { return "ratelimit:" + cmd + ":" + userID }
	dailyCmdTrack       = func(cmd string) string { return "tracking:daily:" + cmd }
	fishInvKey          = func(userID string) string { return "fish:" + userID }
	gatherBaitKey       = func(userID string) string { return "user:gatherbait:" + userID }
	globalStatsKey      = func(userID string) string { return "statistics:global:" + userID }
	guildStatsKey       = func(userID, guildID string) string { return "statistics:" + guildID + ":" + userID }
	hourlyCmdTrack      = func(cmd string) string { return "tracking:hourly:" + cmd }
	inventoryKey        = func(userID string) string { return "user:inventory:" + userID }
	locationDensityKey  = func(userID string) string { return "user:locationdensity:" + userID }
	locationKey         = func(userID string) string { return "user:location:" + userID }
	noInvEEKey          = func(userID string) string { return "ee:" + userID }
	ownedItemKey        = func(userID, item string) string { return fmt.Sprintf("user:inventory:%s:%s", userID, item) }
	scoreGuildKey       = func(guildID string) string { return "exp:guild:" + guildID }
	totalCmdTrack       = func(cmd string) string { return "tracking:total:" + cmd }
	userTrackKey        = func(userID string) string { return "user:" + userID }
)

const (
	gatherBaitTimeout = 6 * time.Hour
	scoreGlobalKey    = "exp:global"
)
