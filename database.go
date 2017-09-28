package main

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

	elastic "gopkg.in/olivere/elastic.v5"
	elogrus "gopkg.in/sohlich/elogrus.v2"

	"github.com/ThyLeader/discordrus"
	"github.com/go-redis/redis"
	"github.com/iopred/discordgo"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

var redisClient *redis.Client

const locDensityExpiration time.Duration = 3 * time.Hour

func init() {
	GetConfigs()
	client, err := elastic.NewClient(elastic.SetURL("http://10.0.0.2:9200"))
	if err != nil {
		log.Panic(err)
	}
	hook, err := elogrus.NewElasticHook(client, "localhost", log.DebugLevel, "fishy-dev")
	if err != nil {
		log.Panic(err)
	}
	log.AddHook(hook)
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
			DisableInlineFields: true,
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

func logInfo(ctx string, err error) {
	pc, _, _, _ := runtime.Caller(1)
	log.WithFields(log.Fields{
		"Error":    err.Error(),
		"Function": runtime.FuncForPC(pc).Name(),
	}).Info(ctx)
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

// DBSetLocDensity randomly assigns density to a new location after fishing
func DBSetLocDensity(location string, userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	key := LocDensityKey(userID)
	cmd := redisClient.Get(key).Val()
	err := json.Unmarshal([]byte(cmd), &LocDensity)
	if err != nil {
		return UserLocDensity{}, err
	}

	r1, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		logError("error generating random number", err)
		return UserLocDensity{}, err
	}
	r2, err := rand.Int(rand.Reader, big.NewInt(99))
	if err != nil {
		logError("error generating random number", err)
		return UserLocDensity{}, err
	}
	randDensity := int(r1.Int64()) + 1
	randLocation := int(r2.Int64())

	fmt.Println(randDensity, randLocation)
	switch location {
	case "lake":
		LocDensity.Lake -= randDensity
		if randLocation < 50 {
			LocDensity.River += randDensity
		} else {
			LocDensity.Ocean += randDensity
		}
	case "river":
		LocDensity.River -= randDensity
		if randLocation < 50 {
			LocDensity.Lake += randDensity
		} else {
			LocDensity.Ocean += randDensity
		}
	case "ocean":
		LocDensity.Ocean -= randDensity
		if randLocation < 50 {
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
func DBGetSetLocDensity(location string, userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	var err error

	LocDensity, err = DBGetLocDensity(userID)
	if err != nil {
		return UserLocDensity{}, err
	}

	_, err = DBSetLocDensity(location, userID)
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
func DBGetBiteRate(userID string, locDen UserLocDensity, loc string) int64 {
	switch loc {
	case "lake":
		return calcBiteRate(int64(locDen.Lake))

	case "river":
		return calcBiteRate(int64(locDen.River))

	case "ocean":
		return calcBiteRate(int64(locDen.Ocean))
	}
	log.WithFields(log.Fields{
		"User":     userID,
		"Location": loc,
	}).Error("User does not have a known location")
	return 0
}

//
func DBGetCatchRate(userID string) (int64, error) {
	inv := DBGetInventory(userID)
	switch inv.Rod.Current {
	case 200:
		return 50, nil
	case 201:
		return 55, nil
	case 202:
		return 60, nil
	case 203:
		return 70, nil
	case 204:
		return 80, nil
	}
	return 50, nil
}

//
func DBGetFishRate(userID string) (int64, error) {
	inv := DBGetInventory(userID)
	switch inv.Hook.Current {
	case 300:
		return 50, nil
	case 301:
		return 60, nil
	case 302:
		return 70, nil
	case 303:
		return 80, nil
	case 304:
		return 90, nil
	}
	return 50, nil
}

// DBGetInventory returns a users inventory tiers
func DBGetInventory(userID string) UserItems {
	var items UserItems
	conv := map[string]map[string]interface{}{}
	key := InventoryKey(userID)
	if DBInventoryCheckExists(userID) {
		keys, err := redisClient.HGetAll(key).Result()
		if err != nil {
			logError("Unable to retrieve inventory", err)
			return UserItems{}
		}
		for i, e := range keys {
			c, err := strconv.Atoi(e)
			if err != nil {
				logInfo("Unable to convert inventory tier to int", err)
				redisClient.HDel(key, i)
				continue
			}
			conv[i] = map[string]interface{}{"current": c, "owned": DBGetOwnedItems(userID, i)}
		}
		err = mapstructure.Decode(conv, &items)
		if err != nil {
			logError("Unable to decode inventory map", err)
			return UserItems{}
		}
		return items
	}
	return UserItems{}
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
		logError("Unable to increment global exp", err)
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
	if allowedItems[item] {
		return redisClient.HSet(InventoryKey(userID), item, tier).Err()
	}
	return fmt.Errorf("Item %s not allowed", item)
}

var allowedItems = map[string]bool{
	"rod":     true,
	"hook":    true,
	"vehicle": true,
	"baitbox": true,
	"bait":    true,
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
	var conv = map[string]interface{}{}
	key := GlobalStatsKey(userID)
	if keyExists(key) {
		data := redisClient.HGetAll(key).Val()
		for i, e := range data {
			switch strings.ToLower(i) {
			case "garbage", "fish", "casts":
				c, err := strconv.Atoi(e)
				if err != nil {
					logError("error converting stat value to int", err)
					redisClient.HSet(key, i, 0)
					conv[i] = 0
					continue
				}
				conv[i] = c
			case "avglength":
				c, err := strconv.ParseFloat(e, 64)
				if err != nil {
					logError("error converting stat value to int", err)
					redisClient.HSet(key, i, 0)
					conv[i] = float64(0)
					continue
				}
				conv[i] = c
			}
		}
		err := mapstructure.Decode(conv, &stats)
		if err != nil {
			logError("Unable to decode guild stats map", err)
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
	var conv = map[string]interface{}{}
	key := GuildStatsKey(userID, guildID)
	if keyExists(key) {
		data := redisClient.HGetAll(key).Val()
		for i, e := range data {
			switch strings.ToLower(i) {
			case "garbage", "fish", "casts":
				c, err := strconv.Atoi(e)
				if err != nil {
					logError("error converting stat value to int", err)
					redisClient.HSet(key, i, 0)
					conv[i] = 0
					continue
				}
				conv[i] = c
			case "avglength":
				c, err := strconv.ParseFloat(e, 64)
				if err != nil {
					logError("error converting stat value to int", err)
					redisClient.HSet(key, i, 0)
					conv[i] = float64(0)
					continue
				}
				conv[i] = c
			}
		}
		err := mapstructure.Decode(conv, &stats)
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
	err := redisClient.HIncrBy(GuildStatsKey(userID, guildID), "garbage", 1).Err()
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
	stats := DBGetGlobalStats(userID)

	totL := float64(stats.Fish) * float64(stats.AvgLength)
	stats.Fish++
	totL += len
	stats.AvgLength = totL / float64(stats.Fish)

	err := redisClient.HSet(key, "fish", stats.Fish).Err()
	if err != nil {
		logError("Unable to set new fish stat", err)
		return
	}
	err = redisClient.HSet(key, "avgLength", stats.AvgLength).Err()
	if err != nil {
		logError("Unable to set new avgLength stat", err)
		return
	}
}

//
func DBIncrGuildAvgFishStats(userID, guildID string, len float64) {
	key := GuildStatsKey(userID, guildID)
	stats := DBGetGuildStats(userID, guildID)

	totL := float64(stats.Fish) * float64(stats.AvgLength)
	stats.Fish++
	totL += len
	stats.AvgLength = totL / float64(stats.Fish)

	err := redisClient.HSet(key, "fish", stats.Fish).Err()
	if err != nil {
		logError("Unable to set new fish total fish", err)
		return
	}
	err = redisClient.HSet(key, "avgLength", stats.AvgLength).Err()
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
	conv := map[string]int{}
	inv := FishInv{}
	keys := redisClient.HGetAll(key).Val()
	for i, e := range keys {
		c, err := strconv.Atoi(e)
		if err != nil {
			logError("Unable to convert fish stats to int", err)
			return FishInv{0, 0, 0, 0}
		}
		conv[i] = c
	}
	mapstructure.Decode(conv, &inv)
	return inv
}

//
func DBAddFishToInv(userID, catchType string, worth float64) error {
	key := FishInvKey(userID)

	if DBGetInvSize(userID) >= DBGetInvCapacity(userID) {
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
	//fmt.Println(fish, legendary)
	return fish + legendary
}

//
func DBSellFish(userID string) map[string]string {
	key := FishInvKey(userID)
	fish := redisClient.HGetAll(key).Val()
	redisClient.HMSet(key, map[string]interface{}{"fish": 0, "garbage": 0, "legendaries": 0, "worth": 0})
	return fish
}

//
func DBGetInvCapacity(userID string) int {
	inv := DBGetInventory(userID)
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

//
func DBGetBaitCapacity(userID string) int {
	inv := DBGetInventory(userID)
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
				logInfo("Fixing broken bait type amt", errors.New("unable to convert bait tier amount to int"))
				redisClient.HSet(key, i, 0)
				conv["t"+i] = 0
				continue
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
func DBAddBait(userID string, tier, amt int) (int, int64, error) {
	cur, err := DBGetBaitTierAmount(userID, tier)
	if err != nil {
		logError("Unable to get current bait tier amount", err)
		return -1, -1, err
	}
	cap := DBGetBaitCapacity(userID)

	if cur+amt > cap && amt != -1 {
		return -1, -1, fmt.Errorf("%v exceeds the bait limit of %v", cur+amt, cap)
	}
	tot, err := redisClient.HIncrBy(BaitInvKey(userID), strconv.Itoa(tier), int64(amt)).Result()
	return cur, tot, err
}

//
func DBGetBaitTierAmount(userID string, tier int) (int, error) {
	key := BaitInvKey(userID)
	if keyExists(key) {
		if a := redisClient.HGet(BaitInvKey(userID), strconv.Itoa(tier)).Val(); a != "" {
			return strconv.Atoi(a)
		}
	}
	DBSetBaitDefault(userID)
	return 0, nil
}

//
func DBSetBaitDefault(userID string) BaitInv {
	d := map[string]interface{}{
		"1": 0,
		"2": 0,
		"3": 0,
		"4": 0,
		"5": 0,
	}
	redisClient.HMSet(BaitInvKey(userID), d)
	return BaitInv{0, 0, 0, 0, 0}
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
func DBGetCurrentBaitAmt(userID string) (int, error) {
	tier := DBGetCurrentBaitTier(userID)
	key := BaitInvKey(userID)
	n, err := strconv.Atoi(redisClient.HGet(key, fmt.Sprintf("%v", tier)).Val())
	if err != nil {
		redisClient.HSet(key, fmt.Sprintf("%v", tier), 0)
		return DBGetCurrentBaitAmt(userID)
	}
	return n, nil
}

//
func DBLoseBait(userID string) (int, error) {
	_, rem, err := DBAddBait(userID, DBGetCurrentBaitTier(userID), -1)
	if err != nil {
		logError("Error subtracting bait after successful catch", err)
		return -1, errors.New("Error subtracting bait")
	}
	return int(rem), nil
}

//
func DBGetOwnedItems(userID, item string) []int {
	key := OwnedItemKey(userID, item)
	if keyExists(key) {
		conv := []int{}
		owned, err := redisClient.SMembers(key).Result()
		if err != nil {
			logError("error retrieving owned items", err)
			return []int{}
		}
		for _, e := range owned {
			if e != "" {
				c, err := strconv.Atoi(e)
				if err != nil {
					logError("unable to convert owned item to int", err)
					continue
				}
				conv = append(conv, c)
			}
		}
		return conv
	}
	return []int{}
}

//
func DBEditOwnedItems(userID, item string, items []int) error {
	conv := []interface{}{}
	for _, e := range items {
		conv = append(conv, strconv.Itoa(e))
	}
	return redisClient.SAdd(OwnedItemKey(userID, item), conv...).Err()
}

// this is useless but i wanna keep it cuz it looks cool
func highestBait(bait BaitInv) int {
	h := 0
	v := reflect.ValueOf(bait)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Interface().(int) > 0 {
			n, err := strconv.Atoi(string(f.Type().Name()[1]))
			if err != nil {
				logError("Error converting bait tier to int", err)
				return -1
			}
			if n > h {
				h = n
			}
		}
	}
	if h == 0 {
		logError(fmt.Sprintf("%+v", bait), errors.New("Could not find highest bait"))
	}
	return h
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

func calcBiteRate(density int64) (rate int64) {
	if density == 100 {
		rate = 50
		return
	}

	if density < 100 {
		rate = int64((float32(0.4) * float32(density)) + 10.0)
		return
	}

	if density > 100 {
		rate = int64((float32(0.25) * float32(density)) + 25.0)
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
