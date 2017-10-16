package database

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/iopred/discordgo"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

const locDensityExpiration time.Duration = 3 * time.Hour // TODO: conf var

func logError(msg string, err error) {
	pc, _, _, _ := runtime.Caller(1)
	log.WithFields(log.Fields{
		"error": err,
		"in":    runtime.FuncForPC(pc).Name(),
	}).Error(msg)
}

// GetLocDensity will get the current location density or set to default if not in the database.
func GetLocDensity(userID string) (UserLocDensity, error) {
	var locDensity UserLocDensity
	key := locationDensityKey(userID)

	if keyExists(key) {
		// Get key and unmarshal into LocDensity
		cmd := redisClient.Get(key).Val()
		if err := json.Unmarshal([]byte(cmd), &locDensity); err != nil {
			return UserLocDensity{}, err
		}
	} else {
		// Set default location density in database and return it
		locDensity = UserLocDensity{100, 100, 100}
		if err := marshalAndSet(locDensity, key, locDensityExpiration); err != nil {
			return UserLocDensity{}, err
		}
	}
	return locDensity, nil
}

// IncreaseRandomLocDensity randomly assigns density to a random location (that isn't `location`).
func IncreaseRandomLocDensity(location string, userID string) (UserLocDensity, error) {
	// Get current location density
	locDensity, err := GetLocDensity(userID)
	if err != nil {
		return locDensity, err
	}

	// Generate random density increase value and pick a random location
	r1, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		logError("error generating random number r1", err)
		return UserLocDensity{}, err
	}
	r2, err := rand.Int(rand.Reader, big.NewInt(99))
	if err != nil {
		logError("error generating random number r2", err)
		return UserLocDensity{}, err
	}
	randDensity := int(r1.Int64()) + 1
	randLocation := int(r2.Int64())
	loc := ""

	// TODO: clean this up, it's really messy and untidy
	switch location {
	case "lake":
		locDensity.Lake -= randDensity
		if randLocation < 50 {
			locDensity.River += randDensity
			loc = "river"
		} else {
			locDensity.Ocean += randDensity
			loc = "ocean"
		}
	case "river":
		locDensity.River -= randDensity
		if randLocation < 50 {
			locDensity.Lake += randDensity
			loc = "lake"
		} else {
			locDensity.Ocean += randDensity
			loc = "ocean"
		}
	case "ocean":
		locDensity.Ocean -= randDensity
		if randLocation < 50 {
			locDensity.Lake += randDensity
			loc = "lake"
		} else {
			locDensity.River += randDensity
			loc = "river"
		}
	default:
		return UserLocDensity{}, errors.New("Invalid Location")
	}

	log.WithFields(log.Fields{
		"user":             userID,
		"rand-density":     randDensity,
		"rand-location":    loc,
		"current-location": location,
	}).Debug("loc-density-change")

	return locDensity, marshalAndSet(locDensity, locationDensityKey(userID), locDensityExpiration)
}

// CheckCommandRateLimit checks the rate limit status of a given command for a user.
func CheckCommandRateLimit(cmd string, userID string) (bool, time.Duration) {
	key := commandRateLimitKey(cmd, userID)
	timeRemaining, _ := redisClient.TTL(key).Result()
	if time.Duration(0)*time.Second >= timeRemaining {
		return false, 0
	}
	return true, timeRemaining
}

// SetRateLimit sets the rate limit duration for a given command.
func SetRateLimit(cmd string, userID string, duration time.Duration) error {
	key := commandRateLimitKey(cmd, userID)
	return redisClient.Set(key, "", duration).Err()
}

// GetLocation returns a user's current location.
func GetLocation(userID string) (string, error) {
	key := locationKey(userID)
	cmd, err := redisClient.Get(key).Result()
	if err != nil {
		// set default location if no key exists
		if err2 := SetLocation(userID, "lake"); err2 != nil {
			logError("error setting default location key", err2)
			return "", err2
		}
		return "lake", nil
	}
	return cmd, nil
}

// SetLocation sets a user's current location.
func SetLocation(userID string, loc string) error {
	return redisClient.Set(locationKey(userID), loc, 0).Err()
}

// GetInventory returns a user's inventory tiers.
// TODO: tidy
func GetInventory(userID string) (UserItems, error) {
	var items UserItems
	conv := map[string]map[string]interface{}{}
	key := inventoryKey(userID)

	if InventoryCheckExists(userID) {
		keys, err := redisClient.HGetAll(key).Result()
		if err != nil {
			return items, err
		}

		for i, e := range keys {
			c, err := strconv.Atoi(e)
			if err != nil {
				log.WithFields(log.Fields{
					"err":   err,
					"value": e,
				}).Warn("unable to convert inventory tier to int")
				redisClient.HDel(key, i)
				continue
			}
			owned, err := GetOwnedItems(userID, i)
			if err != nil {
				return items, err
			}
			conv[i] = map[string]interface{}{"current": c, "owned": owned}
		}

		if err = mapstructure.Decode(conv, &items); err != nil {
			return items, err
		}
		return items, nil
	}
	return defaultUserItems, nil
}

// InventoryCheckExists makes sure a user has an inventory key before modifying it.
func InventoryCheckExists(userID string) bool {
	key := inventoryKey(userID)
	if keyExists(key) {
		return true
	}
	redisClient.HMSet(key, map[string]interface{}{
		"bait":    0,
		"rod":     0,
		"hook":    0,
		"vehicle": 0,
		"baitbox": 0,
	})
	return false
}

// GetGlobalScore gets a user's global experience (score).
func GetGlobalScore(userID string) float64 {
	exp, err := redisClient.ZScore(scoreGlobalKey, userID).Result()
	if err != nil {
		z := redis.Z{Score: 0, Member: userID}
		redisClient.ZAdd(scoreGlobalKey, z)
		return float64(0)
	}
	return exp
}

// IncrementGlobalScore increments a user's global experience (score) by `amt`.
func IncrementGlobalScore(userID string, amt float64) error {
	err := redisClient.ZIncrBy(scoreGlobalKey, amt, userID).Err()
	if err != nil {
		log.WithFields(log.Fields{
			"amt":    amt,
			"err":    err,
			"userID": userID,
		}).Error("failed to increment global exp for a user")
		return err
	}
	return nil
}

// GetGlobalScorePage gets a specific page of global scores.
func GetGlobalScorePage(p int) ([]redis.Z, error) {
	if p == 1 {
		return redisClient.ZRevRangeWithScores(scoreGlobalKey, 0, 9).Result()
	}
	return redisClient.ZRevRangeWithScores(scoreGlobalKey, int64(p-1)*10, int64(p*10)-1).Result()
}

// GetGlobalScoreRank returns a user's global score ranking.
func GetGlobalScoreRank(u string) (int64, float64) {
	return redisClient.ZRevRank(scoreGlobalKey, u).Val(), redisClient.ZScore(scoreGlobalKey, u).Val()
}

// GetGuildScore gets a user's guild experience (score).
func GetGuildScore(userID string, guildID string) float64 {
	exp, err := redisClient.ZScore(scoreGuildKey(guildID), userID).Result()
	if err != nil {
		z := redis.Z{Score: 0, Member: userID}
		redisClient.ZAdd(scoreGuildKey(guildID), z)
		return float64(0)
	}
	return exp
}

// IncrementGuildScore increments a user's global experience (score).
func IncrementGuildScore(userID string, amt float64, guildID string) error {
	err := redisClient.ZIncrBy(scoreGuildKey(guildID), amt, userID).Err()
	if err != nil {
		log.WithFields(log.Fields{
			"amt":     amt,
			"err":     err,
			"guildID": guildID,
			"userID":  userID,
		}).Error("failed to increment guild score")
		return err
	}
	return nil
}

// GetGuildScorePage gets a specific page of guild scores ub a guild.
func GetGuildScorePage(g string, p int) ([]redis.Z, error) {
	if p == 1 {
		return redisClient.ZRevRangeWithScores(scoreGuildKey(g), 1, 10).Result()
	}
	return redisClient.ZRevRangeWithScores(scoreGuildKey(g), int64(p*10)+1, int64(p+1)*10).Result()
}

// GetGuildScoreRank returns a user's guild score ranking.
func GetGuildScoreRank(u string, g string) (int64, float64) {
	return redisClient.ZRevRank(scoreGuildKey(g), u).Val(), redisClient.ZScore(scoreGuildKey(g), u).Val()
}

// GetItemTier gets a user's specific item tier.
func GetItemTier(userID string, item string) int {
	InventoryCheckExists(userID)
	tier, err := strconv.Atoi(redisClient.HGet(inventoryKey(userID), item).Val())
	if err != nil {
		log.WithFields(log.Fields{
			"err":    err,
			"item":   item,
			"userID": userID,
		}).Error("failed to convert item tier to int")
		return 0
	}
	return tier
}

// EditItemTier changes a user's item tier for a specific item without checking for proper tier progression.
func EditItemTier(userID string, item string, tier string) error {
	InventoryCheckExists(userID)
	if _, ok := allowedItems[item]; ok {
		return redisClient.HSet(inventoryKey(userID), item, tier).Err()
	}
	return fmt.Errorf("invalid item: %s", item)
}

// EditItemTiersSafe changes a user's item tiers and checks for proper progression.
func EditItemTiersSafe(userID string, tiers map[string]string) error {
	inv, err := GetInventory(userID)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(inv)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		for item, tier := range tiers {
			fi := typ.Field(i)
			if tagv := fi.Tag.Get("json"); tagv == item {
				currentTier, _ := strconv.Atoi(v.Field(i).Interface().(string))
				newTier, _ := strconv.Atoi(tier)
				if currentTier != newTier-1 {
					return errors.New("user does not own prior tier of " + item)
				}
				if err = EditItemTier(userID, item, tier); err != nil {
					return err
				}
			}
		}
	}
	return errors.New("item not found")
}

// EditItemTiersUnsafe changes a user's item tiers and does not check for proper progression.
func EditItemTiersUnsafe(userID string, tiers map[string]string) error {
	inv, err := GetInventory(userID)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(inv)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		for item, tier := range tiers {
			fi := typ.Field(i)
			if tagv := fi.Tag.Get("json"); tagv == item {
				err = EditItemTier(userID, item, tier)
				if err != nil {
					return err
				}
			}
		}
	}
	return errors.New("item not found")
}

// CheckMissingInventory returns a list of items a user does not own that the user cannot fish without.
func CheckMissingInventory(userID string) []string {
	if InventoryCheckExists(userID) {
		return []string{"rod", "hook"}
	}

	var items []string
	inv := redisClient.HGetAll(inventoryKey(userID)).Val()
	for k, v := range inv {
		if v == "0" && (k == "rod" || k == "hook") {
			items = append(items, k)
		}
	}
	return items
}

// BlackListUser blacklists a user from using fishy.
func BlackListUser(userID string) {
	redisClient.Set(blackListKey(userID), "", 0)
}

// UnblackListUser removes a user from the blacklist.
func UnblackListUser(userID string) {
	redisClient.Del(blackListKey(userID), "")
}

// CheckBlacklist checks if a user is blacklisted.
func CheckBlacklist(userID string) bool {
	return keyExists(blackListKey(userID))
}

// StartGatherBait starts the bait gathering timeout for a user.
func StartGatherBait(userID string) error {
	return redisClient.Set(gatherBaitKey(userID), "", gatherBaitTimeout).Err()
}

// CheckGatherBait checks to see whether or not a user is currently gathering bait.
func CheckGatherBait(userID string) (bool, time.Duration) {
	timeRemaining := redisClient.TTL(gatherBaitKey(userID)).Val()
	if time.Duration(0)*time.Second >= timeRemaining {
		return false, time.Duration(0)
	}
	return true, timeRemaining
}

// TrackUser tracks a name, discriminator and avatar associated with a given user ID.
func TrackUser(user *discordgo.User) {
	redisClient.HMSet(userTrackKey(user.ID), map[string]interface{}{
		"name":          user.Username,
		"discriminator": user.Discriminator,
		"avatar":        discordgo.EndpointUserAvatar(user.ID, user.Avatar),
	})
}

// GetTrackedUser returns the username and discriminator of a user by ID.
func GetTrackedUser(userID string) string {
	user, err := redisClient.HMGet(userTrackKey(userID), "name", "discriminator").Result()
	if err != nil {
		logError("unable to retrieve tracked user username+discriminator", err)
		return ""
	}
	return fmt.Sprintf("%v#%v", user[0], user[1])
}

// GetTrackedUserAvatar returns the avatar URL for the avatar of a tracked user.
func GetTrackedUserAvatar(userID string) string {
	avatar, err := redisClient.HGet(userTrackKey(userID), "avatar").Result()
	if err != nil {
		logError("unable to retrieve tracked user avatar", err)
		return ""
	}
	return avatar
}

func IncrementInvEE(userID string) {
	redisClient.Incr(noInvEEKey(userID))
}

func GetInvEE(userID string) int {
	e, _ := strconv.Atoi(redisClient.Get(noInvEEKey(userID)).Val())
	return e
}

// GetGlobalStats gets a user's global statistics.
func GetGlobalStats(userID string) (UserStats, error) {
	var stats UserStats
	var conv = map[string]interface{}{}
	key := globalStatsKey(userID)

	if keyExists(key) {
		data := redisClient.HGetAll(key).Val()
		for i, e := range data {
			switch strings.ToLower(i) {
			case "garbage", "fish", "casts":
				c, err := strconv.Atoi(e)
				if err != nil {
					log.WithFields(log.Fields{
						"err":   err,
						"value": e,
					}).Warn("unable to convert stat value to int")
					redisClient.HSet(key, i, 0)
					conv[i] = 0
					continue
				}
				conv[i] = c
			case "avglength":
				c, err := strconv.ParseFloat(e, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err":   err,
						"value": e,
					}).Warn("unable to convert stat value to int")
					redisClient.HSet(key, i, 0)
					conv[i] = float64(0)
					continue
				}
				conv[i] = c
			}
		}

		err := mapstructure.Decode(conv, &stats)
		return stats, err
	}
	redisClient.HMSet(key, map[string]interface{}{"garbage": 0, "fish": 0, "avgLength": 0, "casts": 0})
	return stats, nil
}

// GetGuildStats gets a user's guild statistics.
func GetGuildStats(userID, guildID string) (UserStats, error) {
	var stats UserStats
	var conv = map[string]interface{}{}
	key := guildStatsKey(userID, guildID)

	if keyExists(key) {
		data := redisClient.HGetAll(key).Val()
		for i, e := range data {
			switch strings.ToLower(i) {
			case "garbage", "fish", "casts":
				c, err := strconv.Atoi(e)
				if err != nil {
					log.WithFields(log.Fields{
						"err":   err,
						"value": e,
					}).Warn("unable to convert stat value to int")
					redisClient.HSet(key, i, 0)
					conv[i] = 0
					continue
				}
				conv[i] = c
			case "avglength":
				c, err := strconv.ParseFloat(e, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err":   err,
						"value": e,
					}).Warn("unable to convert stat value to int")
					redisClient.HSet(key, i, 0)
					conv[i] = float64(0)
					continue
				}
				conv[i] = c
			}
		}

		err := mapstructure.Decode(conv, &stats)
		return stats, err
	}
	redisClient.HMSet(key, map[string]interface{}{"garbage": 0, "fish": 0, "avgLength": 0, "casts": 0})
	return stats, nil
}

// IncrementGlobalCastStats adds one to a user's global cast statistics.
func IncrementGlobalCastStats(userID string) error {
	return redisClient.HIncrBy(globalStatsKey(userID), "casts", 1).Err()
}

// IncrementGuildCastStats adds one to a user's guild cast statistics.
func IncrementGuildCastStats(userID, guildID string) error {
	return redisClient.HIncrBy(guildStatsKey(userID, guildID), "casts", 1).Err()
}

// IncrementGlobalGarbageStats adds one to a user's global garbage statistics.
func IncrementGlobalGarbageStats(userID string) error {
	return redisClient.HIncrBy(globalStatsKey(userID), "garbage", 1).Err()
}

// IncrementGuildGarbageStats adds one to a user's guild garbage statistics.
func IncrementGuildGarbageStats(userID, guildID string) error {
	return redisClient.HIncrBy(guildStatsKey(userID, guildID), "garbage", 1).Err()
}

// AddGlobalAvgFishStats updates a user's global caught fish count and avgLength statistics.
func AddGlobalAvgFishStats(userID string, len float64) error {
	stats, err := GetGlobalStats(userID)
	if err != nil {
		return err
	}

	totalLength := float64(stats.Fish) * float64(stats.AvgLength)
	stats.Fish++
	totalLength += len
	stats.AvgLength = totalLength / float64(stats.Fish)

	key := globalStatsKey(userID)
	if err := redisClient.HSet(key, "fish", stats.Fish).Err(); err != nil {
		return err
	}
	return redisClient.HSet(key, "avgLength", stats.AvgLength).Err()
}

// AddGuildAvgFishStats updates a user's guild caught fish count and avgLength statistics.
func AddGuildAvgFishStats(userID, guildID string, len float64) error {
	stats, err := GetGuildStats(userID, guildID)
	if err != nil {
		return err
	}

	totalLength := float64(stats.Fish) * float64(stats.AvgLength)
	stats.Fish++
	totalLength += len
	stats.AvgLength = totalLength / float64(stats.Fish)

	key := guildStatsKey(userID, guildID)
	if err := redisClient.HSet(key, "fish", stats.Fish).Err(); err != nil {
		return err
	}
	return redisClient.HSet(key, "avgLength", stats.AvgLength).Err()
}

// GetFishInv returns a user's fish inventory.
func GetFishInv(userID string) (FishInv, error) {
	var inv FishInv
	key := fishInvKey(userID)
	if !keyExists(key) {
		redisClient.HMSet(key, map[string]interface{}{"fish": 0, "garbage": 0, "legendaries": 0, "worth": 0})
		return inv, nil
	}

	conv := map[string]int{}
	keys := redisClient.HGetAll(key).Val()
	for i, e := range keys {
		c, err := strconv.Atoi(e)
		if err != nil {
			log.WithFields(log.Fields{
				"err":   err,
				"key":   key,
				"hKey":  keys,
				"value": e,
			}).Warn("failed to convert fish stats to int")
			return inv, err
		}
		conv[i] = c
	}
	err := mapstructure.Decode(conv, &inv)
	return inv, err
}

// AddFishToInv adds a fish to a user's inventory by type.
func AddFishToInv(userID, catchType string, worth float64) error {
	if GetInvSize(userID) >= GetInvCapacity(userID) {
		return errors.New("Inventory full")
	}

	key := fishInvKey(userID)
	switch catchType {
	case "fish", "garbage", "legendary":
		if err := redisClient.HIncrBy(key, catchType, 1).Err(); err != nil {
			return err
		}
	}
	return redisClient.HIncrBy(key, "worth", int64(worth)).Err()
}

// GetInvSize returns the size of a user's inventory (not to be confused with GetInvCapacity).
func GetInvSize(userID string) int {
	key := fishInvKey(userID)
	fish, _ := strconv.Atoi(redisClient.HGet(key, "fish").Val())
	legendary, _ := strconv.Atoi(redisClient.HGet(key, "legendary").Val())
	return fish + legendary
}

// SellFish empties a user's inventory and returns the previous inventory information.
func SellFish(userID string) (FishInv, error) {
	inv, err := GetFishInv(userID)
	if err != nil {
		return FishInv{}, err
	}
	err = redisClient.HMSet(fishInvKey(userID), map[string]interface{}{
		"fish":        0,
		"garbage":     0,
		"legendaries": 0,
		"worth":       0,
	}).Err()
	if err != nil {
		return FishInv{}, err
	}
	return inv, nil
}

// GetInvCapacity returns the maximum amount of items a user can carry (not including bait, which has a separate
// capacity).
func GetInvCapacity(userID string) int {
	inv, err := GetInventory(userID)
	if err != nil {
		return 25 // default value
	}
	switch inv.Vehicle.Current {
	case 401:
		return 50
	case 402:
		return 100
	case 403:
		return 250
	case 404:
		return 500
	}
	return 25
}

// GetBaitCapacity returns the maximum amount of bait a user can carry.
func GetBaitCapacity(userID string) int {
	inv, err := GetInventory(userID)
	if err != nil {
		return 25
	}
	switch inv.BaitBox.Current {
	case 501:
		return 50
	case 502:
		return 75
	case 503:
		return 100
	case 504:
		return 150
	}
	return 25
}

// GetBaitInv returns the user's bait inventory, setting it to the default inventory if it is invalid or doesn't exist.
func GetBaitInv(userID string) (BaitInv, error) {
	key := baitInvKey(userID)
	conv := map[string]int{}
	var bait BaitInv

	if keyExists(key) {
		inv, err := redisClient.HGetAll(key).Result()
		if err != nil {
			return bait, err
		}
		for i, e := range inv {
			b, err := strconv.Atoi(e)
			if err != nil {
				log.WithFields(log.Fields{
					"err":   err,
					"key":   i,
					"value": e,
				}).Warn("fixing broken bait type amt, setting to 0")
				redisClient.HSet(key, i, 0)
				conv["t"+i] = 0
				continue
			}
			conv["t"+i] = b
		}

		if err = mapstructure.Decode(conv, &bait); err != nil {
			log.WithField("err", err).Warn("unable to decode bait inventory map to struct")
			return BaitInv{}, err
		}
		return bait, nil
	}

	if _, err := SetBaitDefault(userID); err != nil {
		log.WithField("err", err).Warn("unable to set default bait inventory")
		return BaitInv{}, err
	}
	return BaitInv{0, 0, 0, 0, 0}, nil
}

// GetBaitCount returns the amount of bait a user has in their inventory (from all tiers).
func GetBaitCount(userID string) (int, error) {
	bait, err := GetBaitInv(userID)
	if err != nil {
		return 0, err
	}
	return bait.T1 + bait.T2 + bait.T3 + bait.T4 + bait.T5, nil
}

// AddBait adds amt to a user's bait inventory for the specified tier.
func AddBait(userID string, tier, amt int) (int, int64, error) {
	cur, err := GetBaitTierAmount(userID, tier)
	if err != nil {
		log.WithField("err", err).Warn("unable to get current bait tier amount")
		return -1, -1, err
	}
	cap := GetBaitCapacity(userID)

	if cur+amt > cap && amt != -1 {
		return -1, -1, fmt.Errorf("%v exceeds the bait limit of %v", cur+amt, cap)
	}
	tot, err := redisClient.HIncrBy(baitInvKey(userID), strconv.Itoa(tier), int64(amt)).Result()
	return cur, tot, err
}

// GetBaitTierAmount returns the amount of bait in a user's bait inventory for a specific tier.
func GetBaitTierAmount(userID string, tier int) (int, error) {
	key := baitInvKey(userID)
	if keyExists(key) {
		if a := redisClient.HGet(baitInvKey(userID), strconv.Itoa(tier)).Val(); a != "" {
			return strconv.Atoi(a)
		}
	}
	SetBaitDefault(userID)
	return 0, nil
}

// SetBaitDefault sets a user's bait inventory to the default value (0 for every tier).
func SetBaitDefault(userID string) (BaitInv, error) {
	d := map[string]interface{}{
		"1": 0,
		"2": 0,
		"3": 0,
		"4": 0,
		"5": 0,
	}
	return BaitInv{0, 0, 0, 0, 0}, redisClient.HMSet(baitInvKey(userID), d).Err()
}

// GetCurrentBaitTier returns the user's current bait tier.
func GetCurrentBaitTier(userID string) int {
	key := baitTierKey(userID)
	if keyExists(key) {
		val := redisClient.Get(key).Val()
		tier, err := strconv.Atoi(val)
		if err != nil {
			log.WithFields(log.Fields{
				"err":   err,
				"value": val,
			}).Warn("failed to parse current bait tier")
			return 0
		}
		return tier
	}

	if err := SetCurrentBaitTier(userID, 0); err != nil {
		log.WithField("err", err).Warn("failed to set current bait tier to 0")
		return 0
	}
	return 1
}

// SetCurrentBaitTier sets a user's current bait tier to 0.
func SetCurrentBaitTier(userID string, tier float64) error {
	return redisClient.Set(baitTierKey(userID), tier, 0).Err()
}

// GetCurrentBaitAmt returns the amount of bait a user has for their currently selected tier.
func GetCurrentBaitAmt(userID string) (int, error) {
	tier := GetCurrentBaitTier(userID)
	return GetBaitTierAmount(userID, tier)
}

// DecrementBait takes 1 away from a user's currently selected bait tier.
func DecrementBait(userID string) (int, error) {
	_, rem, err := AddBait(userID, GetCurrentBaitTier(userID), -1)
	if err != nil {
		log.WithField("err", err).Warn("failed to decrement bait after successful catch")
		return -1, err
	}
	return int(rem), nil
}

// GetOwnedItems returns a user's owned items.
func GetOwnedItems(userID, item string) ([]int, error) {
	key := ownedItemKey(userID, item)
	if keyExists(key) {
		conv := []int{}
		owned, err := redisClient.SMembers(key).Result()
		if err != nil {
			log.WithField("err", err).Warn("failed to retrieve user's owned items")
			return []int{}, err
		}
		for _, e := range owned {
			if e != "" {
				c, err := strconv.Atoi(e)
				if err != nil {
					log.WithFields(log.Fields{
						"err":   err,
						"value": e,
					}).Warn("failed to convert owned item to int")
					continue
				}
				conv = append(conv, c)
			}
		}
		return conv, nil
	}
	return []int{}, nil
}

// EditOwnedItems replaces a user's owned items with another set of owned items.
func EditOwnedItems(userID, item string, items []int) error {
	conv := []interface{}{}
	for _, e := range items {
		conv = append(conv, strconv.Itoa(e))
	}
	return redisClient.SAdd(ownedItemKey(userID, item), conv...).Err()
}

// GetCmdStats returns the command statistics information for a specific command.
func GetCmdStats(cmd string) (CommandStatData, error) {
	hourlyKey := hourlyCmdTrack(cmd)
	dailyKey := dailyCmdTrack(cmd)
	hour, err := redisClient.ZCard(hourlyKey).Result()
	if err != nil {
		return CommandStatData{}, err
	}
	day, err := redisClient.ZCard(dailyKey).Result()
	if err != nil {
		return CommandStatData{}, err
	}
	tot, err := redisClient.Get(totalCmdTrack(cmd)).Result()
	if err != nil {
		return CommandStatData{}, err
	}
	totS, err := strconv.Atoi(tot)
	if err != nil {
		return CommandStatData{}, err
	}
	return CommandStatData{int(hour), int(day), totS}, nil
}
