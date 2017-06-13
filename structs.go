package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// FishData holds the JSON structure for fish.json
type FishData struct {
	Location struct {
		Lake struct {
			T1 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t1"`
			T2 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t2"`
			T3 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t3"`
			T4 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t4"`
			T5 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t5"`
		} `json:"lake"`
		River struct {
			T1 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t1"`
			T2 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t2"`
			T3 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t3"`
			T4 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t4"`
			T5 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t5"`
		} `json:"river"`
		Ocean struct {
			T1 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t1"`
			T2 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t2"`
			T3 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t3"`
			T4 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t4"`
			T5 []struct {
				Name string   `json:"name"`
				Size []int    `json:"size"`
				Time []string `json:"time"`
				Pun  string   `json:"pun"`
			} `json:"t5"`
		} `json:"ocean"`
	} `json:"location"`
	Trash struct {
		Regular  []string `json:"regular"`
		Treasure []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Worth       int    `json:"worth"`
		} `json:"treasure"`
	} `json:"trash"`
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
	} `json:"bait_box"`
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

//
type BuyItemResponse struct {
	UserItems
	Error bool `json:"error"`
}

// APIResponse is a standard API response
type APIResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var (
	Fish   FishData
	Items  ItemData
	Config ConfigData
	Levels LevelData

	files   = []string{"json/fish.json", "json/items.json", "config.json", "json/levels.json"}
	structs = []interface{}{&Fish, &Items, &Config, &Levels}
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
