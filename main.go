package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

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

var client *redis.Client
var config Config

func init() {
	configData, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Panic("Config not detected in current directory, " + err.Error())
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		log.Panic("Could not unmarshal config, " + err.Error())
	}

	client = redis.NewClient(&redis.Options{
		Addr:     config.Redis.URL,
		Password: config.Redis.Password, // no password set
		DB:       config.Redis.DB,       // use default DB
	})

}

func main() {
	router := NewRouter()

	log.Fatal(http.ListenAndServe(":80", router))
}
