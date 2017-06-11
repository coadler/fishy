package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/mitchellh/mapstructure"
)

var redisClient *redis.Client

const locDensityExpiration time.Duration = 3 * time.Hour

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	GetConfigs()
	redisClient = redis.NewClient(&redis.Options{
		Addr:     Config.Redis.URL,
		Password: Config.Redis.Password,
		DB:       Config.Redis.DB,
	})

}

// DBCmdStats will increment a given command's stats by 1
func DBCmdStats(cmd string) *redis.IntCmd {
	return redisClient.IncrBy("stats:cmds:"+cmd, 1)
}

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
			fmt.Println("error unmarshaling json", err.Error())
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
			fmt.Println("Error setting key", err.Error())
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
	return 0
}

// DBGetInventory returns a users inventory tiers
func DBGetInventory(userID string) UserItems {
	var items UserItems
	key := InventoryKey(userID)
	if DBInventoryExists(userID) {
		keys, err := redisClient.HGetAll(key).Result()
		if err != nil {
			fmt.Println("error getting key ", err.Error())
			return UserItems{}
		}
		err = mapstructure.Decode(keys, &items)
		if err != nil {
			fmt.Println("error decoding map", err.Error())
			return UserItems{}
		}
		return items
	}
	return UserItems{"0", "0", "0", "0", "0"}
}

// DBInventoryExists makes sure a user has an inventory key before modifying it
func DBInventoryExists(userID string) bool {
	key := InventoryKey(userID)
	if keyExists(key) {
		redisClient.HMSet(key, map[string]interface{}{"bait": "0", "rod": "0", "hook": "0", "vehicle": "0", "baitbox": "0"})
		return false
	}
	return true
}

// DBGetGlobalScore gets a users global xp for a specific user
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
		fmt.Println("error incrementing guild exp", err.Error())
		return err
	}
	return nil
}

// DBGetItemTier gets a users specific item tier
func DBGetItemTier(userID string, item string) error {
	DBInventoryExists(userID)
	return redisClient.HMGet(InventoryKey(userID), item).Err()
}

// DBEditItemTier changes a users item tier unsafely (without checking for tier progression)
func DBEditItemTier(userID string, item string, tier string) error {
	DBInventoryExists(userID)
	return redisClient.HSet(InventoryKey(userID), item, tier).Err()
}

// DBEditItemTiersSafe changes a users item tiers and checks for progression
func DBEditItemTiersSafe(userID string, tiers map[string]string) error {
	DBInventoryExists(userID)
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
	DBInventoryExists(userID)
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
	DBInventoryExists(userID)
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
