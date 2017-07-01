package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// FishData holds the JSON structure for fish.json
type FishData struct {
	Location struct {
		Lake []struct {
			Fish []struct {
				Image string      `json:"image"`
				Name  string      `json:"name"`
				Pun   string      `json:"pun"`
				Size  []int       `json:"size"`
				Time  interface{} `json:"time"`
			} `json:"fish"`
		} `json:"lake"`
		Ocean []struct {
			Fish []struct {
				Image string      `json:"image"`
				Name  string      `json:"name"`
				Pun   string      `json:"pun"`
				Size  []int       `json:"size"`
				Time  interface{} `json:"time"`
			} `json:"fish"`
		} `json:"ocean"`
		River []struct {
			Fish []struct {
				Image string      `json:"image"`
				Name  string      `json:"name"`
				Pun   string      `json:"pun"`
				Size  []int       `json:"size"`
				Time  interface{} `json:"time"`
			} `json:"fish"`
		} `json:"river"`
	} `json:"location"`
}

// TrashData stores the data structure for trash data
type TrashData struct {
	Regular struct {
		Text []string `json:"text"`
		User []string `json:"user"`
	} `json:"regular"`
	Treasure []struct {
		Description string `json:"description"`
		Name        string `json:"name"`
		Worth       int    `json:"worth"`
	} `json:"treasure"`
}

// UserFish holds the JSON structure for a users current fish inventory
type UserFish struct {
	Fish []struct {
		Location string `json:"location"`
		Name     string `json:"name"`
		Price    int    `json:"price"`
		Size     int    `json:"size"`
		Tier     int    `json:"tier"`
	} `json:"fish"`
}

// ItemData holds the JSON structure for items.json
type ItemData struct {
	Bait []struct {
		Name   string  `json:"name"`
		Tier   int     `json:"tier"`
		Cost   int     `json:"cost"`
		Effect float64 `json:"effect"`
	} `json:"bait"`
	Rod []struct {
		Name   string  `json:"name"`
		Tier   int     `json:"tier"`
		Cost   int     `json:"cost"`
		Effect float64 `json:"effect"`
	} `json:"rod"`
	Hook []struct {
		Name   string  `json:"name"`
		Tier   int     `json:"tier"`
		Cost   int     `json:"cost"`
		Effect float64 `json:"effect"`
	} `json:"hook"`
	Vehicle []struct {
		Name   string `json:"name"`
		Tier   int    `json:"tier"`
		Cost   int    `json:"cost"`
		Effect int    `json:"effect"`
	} `json:"vehicle"`
	BaitBox []struct {
		Name   string `json:"name"`
		Tier   int    `json:"tier"`
		Cost   int    `json:"cost"`
		Effect int    `json:"effect"`
	} `json:"baitbox"`
}

// UserItems holds the JSON structure for a users items
type UserItems struct {
	Bait    string `json:"bait"`
	Rod     string `json:"rod"`
	Hook    string `json:"hook"`
	Vehicle string `json:"vehicle"`
	BaitBox string `json:"baitbox"`
}

// UserLocDensity stores the location density for each user
type UserLocDensity struct {
	Lake  int `json:"lake"`
	River int `json:"river"`
	Ocean int `json:"ocean"`
}

// LocationResponse holds the JSON structure for the location endpoint
type LocationResponse struct {
	Location string `json:"location"`
	Error    bool   `json:"error"`
}

// ConfigData holds the structure for config.json
type ConfigData struct {
	Redis struct {
		URL      string `json:"url"`
		Password string `json:"password"`
		DB       int    `json:"db"`
	} `json:"redis"`
}

// LevelData holds the data for tier requirements
type LevelData struct {
	T1 int `json:"t1"`
	T2 int `json:"t2"`
	T3 int `json:"t3"`
	T4 int `json:"t4"`
	T5 int `json:"t5"`
}

// BuyItemRequest holds the request structure for buying an item
type BuyItemRequest struct {
	Item string `json:"item"`
	Tier string `json:"tier"`
}

// APIResponse is a standard API response
type APIResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

//
type LeaderboardRequest struct {
	Global    bool   `json:"global"`
	Page      int    `json:"page"`
	User      string `json:"user"`
	GuildID   string `json:"guildid,omitempty"`
	GuildName string `json:"guildname,omitempty"`
}

//
type LeaderboardData struct {
	Scores    []LeaderboardUser
	Rank      int64
	Score     float64
	GuildName string
	Global    bool
}

//
type LeaderboardUser struct {
	Score  float64
	Member interface{}
}

//
type TimeData struct {
	Time    string `json:"time"`
	Morning bool   `json:"morning"`
	Night   bool   `json:"night"`
}

//
type SecretStrings struct {
	InvEE []string `json:"invee"`
}

//
type UserStats struct {
	Garbage   int     `json:"garbage"`
	Fish      int     `json:"fish"`
	AvgLength float32 `json:"avglength"`
	Casts     int     `json:"casts"`
}

//
type Catch struct {
	Tier   int     `json:"tier"`
	Name   string  `json:"name"`
	Sell   int     `json:"sell"`
	Length int     `json:"len"`
	Rand   float64 `json:"rand"`
}

//
type CommandStatData struct {
	Hourly int `json:"hourly"`
	Daily  int `json:"daily"`
	Total  int `json:"total"`
}

var (
	Fish    FishData
	Trash   TrashData
	Items   ItemData
	Config  ConfigData
	Levels  LevelData
	Secrets SecretStrings

	files   = []string{"json/fish.json", "json/items.json", "config.json", "json/levels.json", "json/secretstrings.json", "json/trash.json"}
	structs = []interface{}{&Fish, &Items, &Config, &Levels, &Secrets, &Trash}
)

func GetConfigs() {
	for k, v := range files {
		data, err := ioutil.ReadFile(v)
		if err != nil {
			log.Panic(v + " not detected in current directory, " + err.Error())
		}

		if err := json.Unmarshal(data, &structs[k]); err != nil {
			log.Panic("Could not unmarshal json, " + err.Error())
		}
	}
}
