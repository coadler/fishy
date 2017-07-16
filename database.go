package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/iopred/discordgo"
	"github.com/kz/discordrus"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

var redisClient *redis.Client

// var log = logrus.New()

const locDensityExpiration time.Duration = 3 * time.Hour

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	GetConfigs()
	log.SetLevel(log.DebugLevel)
	log.AddHook(discordrus.NewHook(
		Config.Webhook,
		log.ErrorLevel,
		&discordrus.Opts{
			Username:           "fishy-api1",
			Author:             "",
			DisableTimestamp:   false,
			TimestampFormat:    "Jan 2 15:04:05.00000",
			EnableCustomColors: true,
			CustomLevelColors: &discordrus.LevelColors{
				Debug: 10170623,
				Info:  3581519,
				Warn:  14327864,
				Error: 13631488,
				Panic: 13631488,
				Fatal: 13631488,
			},
			DisableInlineFields: false,
		},
	))
	redisClient = redis.NewClient(&redis.Options{
		Addr:     Config.Redis.URL,
		Password: Config.Redis.Password,
		DB:       Config.Redis.DB,
	})
	if err := redisClient.Ping().Err(); err != nil {
		log.Fatal(err)
	}
}

func logError(ctx string, err error) {
	pc, _, _, _ := runtime.Caller(1)
	log.WithFields(log.Fields{
		"Error":    err.Error(),
		"Function": runtime.FuncForPC(pc).Name(),
	}).Error(ctx)
}

// logError("", err)

// DBGetLocDensity will get current location density or set default if it doesn't exist in the database
func DBGetLocDensity(userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	key := LocDensityKey(userID)
	// check to see if key exists in db (true == exists, false == doesn't exist)
	if keyExists(key) {
		// get key
		cmd := redisClient.Get(key).Val()
		// map key's stored JSON to the UserLocDensity struct
		err := json.Unmarshal([]byte(cmd), &LocDensity)
		if err != nil {
			return UserLocDensity{}, err
		}
	} else {
		// set to default density
		LocDensity = UserLocDensity{100, 100, 100}
		// turn struct into a JSON byte array and set in redis
		err := marshalAndSet(LocDensity, key, locDensityExpiration)
		if err != nil {
			return UserLocDensity{}, err
		}
	}
	return LocDensity, nil
}

// dbSetLocDensity randomly assigns density to a new location after fishing
// note: this should only be called inside of DBGetSetLocDensity and as a result
// does not check to see if the key already exists, therefore it is unexported
// this will return the new location density and an error if applicable
func dbSetLocDensity(location string, userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	key := LocDensityKey(userID)
	cmd := redisClient.Get(key).Val()
	err := json.Unmarshal([]byte(cmd), &LocDensity)
	if err != nil {
		return UserLocDensity{}, err
	}

	randDensity := int(math.Floor(float64(rand.Intn(97)/33))) + 1
	randLocation := rand.Intn(100)
	fmt.Println(randDensity, randLocation)
	switch location {
	case "lake":
		LocDensity.Lake -= randDensity
		if randLocation < 51 {
			LocDensity.River += randDensity
		} else {
			LocDensity.Ocean += randDensity
		}
	case "river":
		LocDensity.River -= randDensity
		if randLocation < 51 {
			LocDensity.Lake += randDensity
		} else {
			LocDensity.Ocean += randDensity
		}
	case "ocean":
		LocDensity.Ocean -= randDensity
		if randLocation < 51 {
			LocDensity.Lake += randDensity
		} else {
			LocDensity.River += randDensity
		}
	default:
		return UserLocDensity{}, errors.New("Invalid Location")
	}

	go marshalAndSet(LocDensity, key, locDensityExpiration)
	return LocDensity, nil
}

// DBGetSetLocDensity returns the location density then sets a new one
// this should be the preferred method for fishing
func DBGetSetLocDensity(location string, userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	var err error

	LocDensity, err = DBGetLocDensity(userID)
	if err != nil {
		return UserLocDensity{}, err
	}

	_, err = dbSetLocDensity(location, userID)
	if err != nil {
		return UserLocDensity{}, err
	}
	return LocDensity, nil
}

// DBCheckRateLimit checks the ratelimit of a given command
func DBCheckRateLimit(cmd string, userID string) (bool, time.Duration) {
	key := RateLimitKey(cmd, userID)
	timeRemaining, _ := redisClient.TTL(key).Result()

	if time.Duration(0)*time.Second >= timeRemaining {
		return false, time.Duration(0)
	}

	return true, timeRemaining
}

// DBSetRateLimit sets a new ratelimit for a given command
func DBSetRateLimit(cmd string, userID string, ttl time.Duration) error {
	key := RateLimitKey(cmd, userID)
	err := redisClient.Set(key, "", ttl).Err()
	if err != nil {
		return err
	}
	return nil
}

// DBGetLocation returns a users current location
func DBGetLocation(userID string) string {
	key := LocationKey(userID)
	cmd, err := redisClient.Get(key).Result()
	if err != nil {
		if err = redisClient.Set(key, "lake", 0).Err(); err != nil { // set default location if no key exists
			logError("Error setting location key", err)
			return ""
		}
		return "lake"
	}
	return cmd
}

// DBSetLocation sets a users location
func DBSetLocation(userID string, loc string) error {
	return redisClient.Set(LocationKey(userID), loc, 0).Err()
}

// DBGetBiteRate returns the biterate for a given user
func DBGetBiteRate(userID string) float32 {
	loc := DBGetLocation(userID)
	locDen, _ := DBGetLocDensity(userID)

	switch loc {
	case "lake":
		return calcBiteRate(float32(locDen.Lake))

	case "river":
		return calcBiteRate(float32(locDen.River))

	case "ocean":
		return calcBiteRate(float32(locDen.Ocean))
	}
	log.WithFields(log.Fields{
		"User":     userID,
		"Location": loc}).Error("User does not have a known location")
	return 0
}

//
func DBGetCatchRate(userID string) (float32, error) {
	rod := redisClient.HGet(InventoryKey(userID), "rod").Val()
	switch rod {
	case "1":
		return .50, nil
	case "2":
		return .55, nil
	case "3":
		return .60, nil
	case "4":
		return .70, nil
	case "5":
		return .80, nil
	}
	log.WithFields(log.Fields{
		"User": userID,
		"Rod":  rod}).Error("Invalid rod tier")
	return 0, errors.New("Invalid rod tier")
}

//
func DBGetFishRate(userID string) (float32, error) {
	hook := redisClient.HGet(InventoryKey(userID), "hook").Val()
	switch hook {
	case "1":
		return .50, nil
	case "2":
		return .60, nil
	case "3":
		return .70, nil
	case "4":
		return .80, nil
	case "5":
		return .90, nil
	}
	log.WithFields(log.Fields{
		"User": userID,
		"Hook": hook}).Error("Invalid hook tier")
	return 0, errors.New("Invalid hook tier")
}

// DBGetInventory returns a users inventory tiers
func DBGetInventory(userID string) UserItems {
	var items UserItems
	conv := map[string]int{}
	key := InventoryKey(userID)
	if DBInventoryCheckExists(userID) {
		keys, err := redisClient.HGetAll(key).Result()
		if err != nil {
			logError("Unable to retrieve inventory", err)
			return UserItems{}
		}
		for i, e := range keys {
			conv[i], err = strconv.Atoi(e)
			if err != nil {
				logError("Unable to convert inventory tier to int", err)
				return UserItems{}
			}
		}
		err = mapstructure.Decode(conv, &items)
		if err != nil {
			logError("Unable to decode inventory map", err)
			return UserItems{}
		}
		return items
	}
	return UserItems{0, 0, 0, 0, 0}
}

// DBInventoryCheckExists makes sure a user has an inventory key before modifying it
func DBInventoryCheckExists(userID string) bool {
	key := InventoryKey(userID)
	if keyExists(key) {
		return true
	}
	redisClient.HMSet(key, map[string]interface{}{"bait": 0, "rod": 0, "hook": 0, "vehicle": 0, "baitbox": 0})
	return false
}

// DBGetGlobalScore gets a users global xp (score) for a specific user
func DBGetGlobalScore(userID string) float64 {
	exp, err := redisClient.ZScore(ScoreGlobalKey, userID).Result()
	if err != nil {
		z := redis.Z{Score: 0, Member: userID}
		redisClient.ZAdd(ScoreGlobalKey, z)
		return float64(0)
	}
	return exp
}

// DBGiveGlobalScore increments a users global exp
func DBGiveGlobalScore(userID string, amt float64) error {
	err := redisClient.ZIncrBy(ScoreGlobalKey, amt, userID).Err()
	if err != nil {
		fmt.Println("error incrementing global exp", err.Error())
		return err
	}
	return nil
}

// DBGetGlobalScorePage gets a specific page of global scores
func DBGetGlobalScorePage(p int) ([]redis.Z, error) {
	if p == 1 {
		return redisClient.ZRevRangeWithScores(ScoreGlobalKey, 0, 9).Result()
	}
	return redisClient.ZRevRangeWithScores(ScoreGlobalKey, int64(p-1)*10, int64(p*10)-1).Result()
}

// DBGetGlobalScoreRank returns a users global score ranking
func DBGetGlobalScoreRank(u string) (int64, float64) {
	return redisClient.ZRevRank(ScoreGlobalKey, u).Val(), redisClient.ZScore(ScoreGlobalKey, u).Val()
}

// DBGetGuildScore gets a users global xp for a specific user
func DBGetGuildScore(userID string, guildID string) float64 {
	exp, err := redisClient.ZScore(ScoreGuildKey(guildID), userID).Result()
	if err != nil {
		z := redis.Z{Score: 0, Member: userID}
		redisClient.ZAdd(ScoreGuildKey(guildID), z)
		return float64(0)
	}
	return exp
}

// DBGiveGuildScore increments a users global exp
func DBGiveGuildScore(userID string, amt float64, guildID string) error {
	err := redisClient.ZIncrBy(ScoreGuildKey(guildID), amt, userID).Err()
	if err != nil {
		logError("Unable to increment guild exp", err)
		return err
	}
	return nil
}

// DBGetGuildScorePage gets a specific page of a guilds scores
func DBGetGuildScorePage(g string, p int) ([]redis.Z, error) {
	if p == 1 {
		return redisClient.ZRevRangeWithScores(ScoreGuildKey(g), 1, 10).Result()
	}
	return redisClient.ZRevRangeWithScores(ScoreGuildKey(g), int64(p*10)+1, int64(p+1)*10).Result()
}

// DBGetGuildScoreRank returns a users guild score ranking
func DBGetGuildScoreRank(u string, g string) (int64, float64) {
	return redisClient.ZRevRank(ScoreGuildKey(g), u).Val(), redisClient.ZScore(ScoreGuildKey(g), u).Val()
}

// DBGetItemTier gets a users specific item tier
func DBGetItemTier(userID string, item string) int {
	DBInventoryCheckExists(userID)
	tier, err := strconv.Atoi(redisClient.HGet(InventoryKey(userID), item).Val())
	if err != nil {
		logError("Unable to convert item tier to int", err)
		return 0
	}
	return tier
}

// DBEditItemTier changes a users item tier unsafely (without checking for tier progression)
func DBEditItemTier(userID string, item string, tier string) error {
	DBInventoryCheckExists(userID)
	return redisClient.HSet(InventoryKey(userID), item, tier).Err()
}

// DBEditItemTiersSafe changes a users item tiers and checks for progression
func DBEditItemTiersSafe(userID string, tiers map[string]string) error {
	DBInventoryCheckExists(userID)
	var err error
	v := reflect.ValueOf(DBGetInventory(userID))
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		for item, tier := range tiers {
			fi := typ.Field(i)
			if tagv := fi.Tag.Get("json"); tagv == item {
				currentTier, _ := strconv.Atoi(v.Field(i).Interface().(string))
				newTier, _ := strconv.Atoi(tier)
				if currentTier != newTier-1 {
					return errors.New("User does not own prior tier of " + item)
				}
				err = DBEditItemTier(userID, item, tier)
				if err != nil {
					return err
				}
			}
		}
	}
	return errors.New("Item not found")
}

// DBEditItemTiersUnsafe changes a users item tiers and does not check for progression
func DBEditItemTiersUnsafe(userID string, tiers map[string]string) error {
	DBInventoryCheckExists(userID)
	var err error
	v := reflect.ValueOf(DBGetInventory(userID))
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		for item, tier := range tiers {
			fi := typ.Field(i)
			if tagv := fi.Tag.Get("json"); tagv == item {
				err = DBEditItemTier(userID, item, tier)
				if err != nil {
					return err
				}
			}
		}
	}
	return errors.New("Item not found")
}

// DBCheckMissingInventory returns a list of items a user does not own that you can't fish without
func DBCheckMissingInventory(userID string) []string {
	DBInventoryCheckExists(userID)
	var items []string
	inv := redisClient.HGetAll(InventoryKey(userID)).Val()
	for k, v := range inv {
		if v == "0" {
			if k == "rod" || k == "hook" {
				items = append(items, k)
			}
		}
	}
	return items
}

// DBBlackListUser sfsafd
func DBBlackListUser(userID string) {
	redisClient.Set(BlackListKey(userID), "", 0)
}

// DBUnblackListUser sfsafd
func DBUnblackListUser(userID string) {
	redisClient.Del(BlackListKey(userID), "")
}

// DBCheckBlacklist checks if a user is blacklisted
func DBCheckBlacklist(userID string) bool {
	return keyExists(BlackListKey(userID))
}

// DBStartGatherBait starts the bait gathering timeout
func DBStartGatherBait(userID string) error {
	return redisClient.Set(GatherBaitKey(userID), "", GatherBaitTimeout).Err()
}

// DBCheckGatherBait checks to see whether or not a user is currently gathering bait
func DBCheckGatherBait(userID string) (bool, time.Duration) {
	key := GatherBaitKey(userID)
	timeRemaining := redisClient.TTL(key).Val()
	if time.Duration(0)*time.Second >= timeRemaining {
		return false, time.Duration(0)
	}
	return true, timeRemaining
}

// DBTrackUser tracks a name, discriminator and avatar associated with a given user id
func DBTrackUser(user *discordgo.User) {
	redisClient.HMSet(UserTrackKey(user.ID), map[string]interface{}{"name": user.Username, "discriminator": user.Discriminator, "avatar": discordgo.EndpointUserAvatar(user.ID, user.Avatar)})
}

// DBGetTrackedUser returns the username and discriminator of a user
func DBGetTrackedUser(userID string) string {
	user, err := redisClient.HMGet(UserTrackKey(userID), "name", "discriminator").Result()
	if err != nil {
		logError("Unable to retrieve tracked user", err)
		return ""
	}
	return fmt.Sprintf("%v#%v", user[0], user[1])
}

// DBGetTrackedUserAvatar returns the URL for the avatar of a tracked user
func DBGetTrackedUserAvatar(userID string) string {
	avatar, err := redisClient.HGet(UserTrackKey(userID), "avatar").Result()
	if err != nil {
		logError("Unable to retrieve tracked user avatar", err)
		return ""
	}
	return avatar
}

// DBIncInvEE [REDACTED]
func DBIncInvEE(userID string) {
	redisClient.Incr(NoInvEEKey(userID))
}

// DBGetInvEE [REDACTED]
func DBGetInvEE(userID string) int {
	e, _ := strconv.Atoi(redisClient.Get(NoInvEEKey(userID)).Val())
	return e
}

// DBGetGlobalStats gets a users global stats
func DBGetGlobalStats(userID string) UserStats {
	var stats UserStats
	key := GlobalStatsKey(userID)
	if keyExists(key) {
		data := redisClient.HGetAll(key).Val()
		err := mapstructure.Decode(data, &stats)
		if err != nil {
			logError("Unable to decode global stats map", err)
			return UserStats{}
		}
		return stats
	}
	redisClient.HMSet(key, map[string]interface{}{"garbage": 0, "fish": 0, "avgLength": 0, "casts": 0})
	return UserStats{0, 0, 0, 0}
}

// DBGetGuildStats gets a users guild stats
func DBGetGuildStats(userID, guildID string) UserStats {
	var stats UserStats
	key := GuildStatsKey(userID, guildID)
	if keyExists(key) {
		data := redisClient.HGetAll(key).Val()
		err := mapstructure.Decode(data, &stats)
		if err != nil {
			logError("Unable to decode guild stats map", err)
			return UserStats{}
		}
		return stats
	}
	redisClient.HMSet(key, map[string]interface{}{"garbage": 0, "fish": 0, "avgLength": 0, "casts": 0})
	return UserStats{0, 0, 0, 0}
}

// DBAddGlobalCast adds one to a users global cast stats
func DBAddGlobalCast(userID string) {
	err := redisClient.HIncrBy(GlobalStatsKey(userID), "casts", 1).Err()
	if err != nil {
		logError("Unable to increment global casts stat", err)
		return
	}
}

// DBAddGuildCast adds one to a users guild cast stats
func DBAddGuildCast(userID, guildID string) {
	err := redisClient.HIncrBy(GuildStatsKey(userID, guildID), "casts", 1).Err()
	if err != nil {
		logError("Unable to increment guild casts stat", err)
		return
	}
}

// DBAddCast adds both guild and global cast stats
func DBAddCast(userID, guildID string) {
	go DBAddGlobalCast(userID)
	go DBAddGuildCast(userID, guildID)
}

// DBAddGlobalGarbage adds one to a users global garbage stats
func DBAddGlobalGarbage(userID string) {
	err := redisClient.HIncrBy(GlobalStatsKey(userID), "garbage", 1).Err()
	if err != nil {
		logError("Unable to increment global garbage stat", err)
		return
	}
}

// DBAddGuildGarbage adds one to a users guild garbage stats
func DBAddGuildGarbage(userID, guildID string) {
	err := redisClient.HIncrBy(GuildStatsKey(userID, guildID), "casts", 1).Err()
	if err != nil {
		logError("Unable to increment guild garbage stat", err)
		return
	}
}

// DBAddGarbage adds both guild and global garbage stats
func DBAddGarbage(userID, guildID string) {
	go DBAddGlobalGarbage(userID)
	go DBAddGuildGarbage(userID, guildID)
}

//
func DBIncrGlobalAvgFishStats(userID string, len float64) {
	key := GlobalStatsKey(userID)
	totF, err := strconv.ParseFloat(redisClient.HGet(key, "fish").Val(), 64)
	if err != nil {
		logError("Unable to parse total fish stat", err)
		return
	}
	avg, err := strconv.ParseFloat(redisClient.HGet(key, "avgLength").Val(), 64)
	if err != nil {
		logError("Unable to parse avg fish length stat", err)
		return
	}
	totL := totF * avg
	totF++
	totL += len
	avg = totL / totF

	err = redisClient.HSet(key, "fish", totF).Err()
	if err != nil {
		logError("Unable to set new fish stat", err)
		return
	}
	err = redisClient.HSet(key, "avgLength", avg).Err()
	if err != nil {
		logError("Unable to set new avgLength stat", err)
		return
	}
}

//
func DBIncrGuildAvgFishStats(userID, guildID string, len float64) {
	key := GuildStatsKey(userID, guildID)
	totF, err := strconv.ParseFloat(redisClient.HGet(key, "fish").Val(), 64)
	if err != nil {
		logError("Unable to parse total fish", err)
		return
	}
	avg, err := strconv.ParseFloat(redisClient.HGet(key, "avgLength").Val(), 64)
	if err != nil {
		logError("Unable to parse avg fish length", err)
		return
	}
	totL := totF * avg
	totF++
	totL += len
	avg = totL / totF

	err = redisClient.HSet(key, "fish", totF).Err()
	if err != nil {
		logError("Unable to set new fish total fish", err)
		return
	}
	err = redisClient.HSet(key, "avgLength", avg).Err()
	if err != nil {
		logError("Unable to set new avgLength stat", err)
		return
	}
}

// DBIncrAvgFishStats increments guild and global fish stats
func DBIncrAvgFishStats(userID, guildID string, len float64) {
	go DBIncrGlobalAvgFishStats(userID, len)
	go DBIncrGuildAvgFishStats(userID, guildID, len)
}

// DBGetFishInv returns an array of catches, representing a user's fish inventory
func DBGetFishInv(userID string) FishInv {
	key := FishInvKey(userID)
	if !keyExists(key) {
		redisClient.HMSet(key, map[string]interface{}{"fish": 0, "garbage": 0, "legendaries": 0, "worth": 0})
		return FishInv{0, 0, 0, 0}
	}
	var inv FishInv
	keys := redisClient.HGetAll(key).Val()
	mapstructure.Decode(keys, &inv)
	return inv
}

//
func DBAddFishToInv(userID, catchType string, worth int) error {
	key := FishInvKey(userID)

	if DBGetInvSize(userID) <= DBGetInvCapacity(userID) {
		return errors.New("Inventory full")
	}

	switch catchType {
	case "fish":
		redisClient.HIncrBy(key, "fish", 1)
	case "garbage":
		redisClient.HIncrBy(key, "garbage", 1)
	case "legendary":
		redisClient.HIncrBy(key, "legendary", 1)
	}
	redisClient.HIncrBy(key, "worth", int64(worth))
	return nil
}

//
func DBGetInvSize(userID string) int {
	key := FishInvKey(userID)
	fish, _ := strconv.Atoi(redisClient.HGet(key, "fish").Val())
	legendary, _ := strconv.Atoi(redisClient.HGet(key, "legendary").Val())
	return fish + legendary
}

//
func DBGetInvCapacity(userID string) int {
	cap, err := strconv.Atoi(redisClient.HGet(InventoryKey(userID), "vehicle").Val())
	if err != nil {
		logError("Unable to convert vehicle tier to int", err)
		return 0
	}
	switch cap {
	case 2:
		return 50
	case 3:
		return 100
	case 4:
		return 250
	case 5:
		return 500
	}
	return 25
}

//
func DBGetBaitCapacity(userID string) int {
	cap, err := strconv.Atoi(redisClient.HGet(InventoryKey(userID), "baitbox").Val())
	if err != nil {
		logError("Unable to convert baitbox tier to int", err)
		return 0
	}
	switch cap {
	case 2:
		return 50
	case 3:
		return 75
	case 4:
		return 100
	case 5:
		return 150
	}
	return 25
}

//
func DBGetBaitInv(userID string) BaitInv {
	key := BaitInvKey(userID)
	conv := map[string]int{}
	var bait BaitInv
	if keyExists(key) {
		inv, err := redisClient.HGetAll(key).Result()
		if err != nil {
			logError("Unable to get bait inventory", err)
			return BaitInv{}
		}
		for i, e := range inv {
			b, err := strconv.Atoi(e)
			if err != nil {
				logError("Unable to convert bait tiers to int", err)
				return BaitInv{}
			}
			conv["t"+i] = b
		}
		err = mapstructure.Decode(conv, &bait)
		if err != nil {
			logError("Unable to decode bait inventory map to struct", err)
			return BaitInv{}
		}
		return bait
	}
	fmt.Println("doesnt exist")
	err := redisClient.HMSet(key, map[string]interface{}{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}).Err()
	if err != nil {
		logError("Unable to set default bait inventory", err)
		return BaitInv{}
	}
	return BaitInv{0, 0, 0, 0, 0}
}

//
func DBGetBaitUsage(userID string) int {
	conv := map[string]int{}
	var bait BaitInv
	key := BaitInvKey(userID)
	inv, err := redisClient.HGetAll(key).Result()
	if err != nil {
		return 0
	}
	for i, e := range inv {
		b, err := strconv.Atoi(e)
		if err != nil {
			logError("Unable to convert bait to int", err)
			return 0
		}
		conv["t"+i] = b
	}
	err = mapstructure.Decode(conv, &bait)
	if err != nil {
		logError("Unable to decode bait inventory map to struct", err)
		return 0
	}
	return bait.T1 + bait.T2 + bait.T3 + bait.T4 + bait.T5
}

//
func DBAddBait(userID string, tier, amt int) (int64, error) {
	return redisClient.HIncrBy(BaitInvKey(userID), strconv.Itoa(tier), int64(amt)).Result()
}

//
func DBGetCurrentBaitTier(userID string) int {
	key := BaitTierKey(userID)
	if keyExists(key) {
		tier, err := strconv.Atoi(redisClient.Get(key).Val())
		if err != nil {
			logError("Unable to parse current bait tier", err)
			return 0
		}
		return tier
	}
	err := redisClient.Set(key, 1, 0).Err()
	if err != nil {
		logError("Unable to set current bait tier", err)
		return 0
	}
	return 1
}

//
func DBSetCurrentBaitTier(userID string, tier float64) error {
	return redisClient.Set(BaitTierKey(userID), tier, 0).Err()
}

//
func DBGetCmdStats(cmd string) (CommandStatData, error) {
	hourlyKey := HourlyCmdTrack(cmd)
	dailyKey := DailyCmdTrack(cmd)
	hour, err := redisClient.ZCard(hourlyKey).Result()
	if err != nil {
		logError("Error retrieving cmd stats", err)
		return CommandStatData{}, err
	}
	day, err := redisClient.ZCard(dailyKey).Result()
	if err != nil {
		logError("Error retrieving cmd stats", err)
		return CommandStatData{}, err
	}
	tot, err := redisClient.Get(TotalCmdTrack(cmd)).Result()
	if err != nil {
		logError("Error retrieving cmd stats", err)
		return CommandStatData{}, err
	}
	totS, err := strconv.Atoi(tot)
	if err != nil {
		logError("Error converting cmd stats to int", err)
		return CommandStatData{}, err
	}
	return CommandStatData{int(hour), int(day), totS}, nil
}

func keyExists(key string) bool {
	return redisClient.Exists(key).Val() == int64(1)
}

func calcBiteRate(density float32) (rate float32) {
	if density == 100 {
		rate = .50
		return
	}

	if density < 100 {
		rate = ((float32(0.4) * density) + 10.0) / 100.0
		return
	}

	if density > 100 {
		rate = ((float32(0.25) * density) + 25.0) / 100.0
		return
	}
	return
}

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
