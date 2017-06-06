package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/go-redis/redis"
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
	if exists := redisClient.Exists(key); exists.Val() == int64(1) {
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

//
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

//
func DBSetLocation(userID string, loc string) error {
	return redisClient.Set(LocationKey(userID), loc, 0).Err()
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
