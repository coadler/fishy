package routes

import (
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
	Prices [][]float64
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

// ItemData holds the JSON structure for items.json
type ItemData struct {
	Bait []struct {
		Name        string  `json:"name"`
		ID          string  `json:"id"`
		Tier        int     `json:"tier"`
		Cost        int     `json:"cost"`
		Effect      float64 `json:"effect"`
		Description string  `json:"description"`
	} `json:"bait"`
	Rod []struct {
		Name        string  `json:"name"`
		ID          string  `json:"id"`
		Tier        int     `json:"tier"`
		Cost        int     `json:"cost"`
		Effect      float64 `json:"effect"`
		Description string  `json:"description"`
	} `json:"rod"`
	Hook []struct {
		Name        string  `json:"name"`
		ID          string  `json:"id"`
		Tier        int     `json:"tier"`
		Cost        int     `json:"cost"`
		Effect      float64 `json:"effect,omitempty"`
		Description string  `json:"description"`
		Modifier    float64 `json:"modifier,omitempty"`
	} `json:"hook"`
	Vehicle []struct {
		Name        string `json:"name"`
		ID          string `json:"id"`
		Tier        int    `json:"tier"`
		Cost        int    `json:"cost"`
		Effect      int    `json:"effect"`
		Description string `json:"description"`
	} `json:"vehicle"`
	BaitBox []struct {
		Name        string `json:"name"`
		ID          string `json:"id"`
		Tier        int    `json:"tier"`
		Cost        int    `json:"cost"`
		Effect      int    `json:"effect"`
		Description string `json:"description"`
	} `json:"bait_box"`
}

// LevelData holds the data for tier requirements
type LevelData struct {
	T1 int `json:"t1"`
	T2 int `json:"t2"`
	T3 int `json:"t3"`
	T4 int `json:"t4"`
	T5 int `json:"t5"`
}

//
type SecretStrings struct {
	InvEE []string `json:"invee"`
}

var (
	fish    FishData
	trash   TrashData
	items   ItemData
	levels  LevelData
	secrets SecretStrings

	configKeys = map[string]interface{}{
		"fish":          &fish,
		"items":         &items,
		"levels":        &levels,
		"secretstrings": &secrets,
		"trash":         &trash,
	}
)

func GetConfigs() {
	for k, v := range configKeys {
		data := viper.GetStringMap(k)

		if err := mapstructure.Decode(data, &v); err != nil {
			log.WithFields(log.Fields{
				"err": err,
				"key": k,
			}).Fatal("failed to marshal config data into config struct")
		}
	}
}
