package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	"github.com/go-redis/redis"
)

// Config holds the structure for config.json
type Config struct {
	Redis struct {
		URL      string `json:"url"`
		Password string `json:"password"`
		DB       int    `json:"db"`
	} `json:"redis"`
}

var redisClient *redis.Client
var config Config

const locDensityExpiration time.Duration = 3 * time.Hour
const locDensityKey string = "user:locationdensity:"

const rateLimitKey string = "ratelimit:"

func init() {
	configData, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Panic("Config not detected in current directory, " + err.Error())
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		log.Panic("Could not unmarshal config, " + err.Error())
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     config.Redis.URL,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})

}

// DBCmdStats will increment a given command's stats by 1
func DBCmdStats(cmd string) *redis.IntCmd {
	return redisClient.IncrBy("stats:cmds:"+cmd, 1)
}

// DBGetLocDensity will get current location density or set default if it doesn't exist in the database
func DBGetLocDensity(userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	key := locDensityKey + userID
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
// does not check to see if the key already exists, therefor it is unexported
// this will return the new location density and an error if applicable
func dbSetLocDensity(location string, userID string) (UserLocDensity, error) {
	var LocDensity UserLocDensity
	key := locDensityKey + userID
	cmd := redisClient.Get(key).Val()
	err := json.Unmarshal([]byte(cmd), &LocDensity)
	if err != nil {
		return UserLocDensity{}, err
	}

	randDensity := rand.Intn(2) + 1
	randLocation := rand.Intn(1) + 1
	switch location {
	case "lake":
		LocDensity.Lake = LocDensity.Lake - randDensity
		if randLocation == 1 {
			LocDensity.River = LocDensity.River + randDensity
		} else {
			LocDensity.Ocean = LocDensity.Ocean + randDensity
		}
	case "river":
		LocDensity.River = LocDensity.River - randDensity
		if randLocation == 1 {
			LocDensity.Lake = LocDensity.Lake + randDensity
		} else {
			LocDensity.Ocean = LocDensity.Ocean + randDensity
		}
	case "ocean":
		LocDensity.Ocean = LocDensity.Ocean - randDensity
		if randLocation == 1 {
			LocDensity.Lake = LocDensity.Lake + randDensity
		} else {
			LocDensity.River = LocDensity.River + randDensity
		}
	default:
		return UserLocDensity{}, errors.New("Invalid Location")
	}

	err = marshalAndSet(LocDensity, key, locDensityExpiration)
	if err != nil {
		return UserLocDensity{}, err
	}
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
	key := rateLimitKey + cmd + ":" + userID
	timeRemaining, _ := redisClient.TTL(key).Result()

	if time.Duration(0)*time.Second > timeRemaining {
		return false, time.Duration(0)
	}

	return true, timeRemaining
}

// DBSetRateLimit sets a new ratelimit for a given command
func DBSetRateLimit(cmd string, userID string, ttl time.Duration) error {
	key := rateLimitKey + cmd + ":" + userID
	err := redisClient.Set(key, "", ttl).Err()
	if err != nil {
		return err
	}
	return nil
}

func marshalAndSet(data interface{}, key string, expiration time.Duration) error {
	set, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = redisClient.Set(key, set, locDensityExpiration).Err()
	if err != nil {
		return err
	}
	return nil
}
