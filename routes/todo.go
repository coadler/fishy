package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// Inventory is the main route for getting a user's item inventory
func Inventory(w http.ResponseWriter, r *http.Request) {
	//go CmdStats("inventory:get", "")
	user := mux.Vars(r)["userID"]

	respond(w,
		map[string]interface{}{
			"items":    DBGetInventory(user),
			"fish":     DBGetFishInv(user),
			"maxFish":  DBGetInvCapacity(user),
			"maxBait":  DBGetBaitCapacity(user),
			"userTier": ExpToTier(DBGetGlobalScore(user)),
		},
	)
}

// Location is the main route for getting and changing or getting a user's location
func Location(w http.ResponseWriter, r *http.Request) {
	var vars = mux.Vars(r)
	var user = vars["userID"]

	if DBCheckBlacklist(user) {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("User blacklisted"),
				"",
			},
		)
		return
	}

	if r.Method == "GET" { // get location
		go CmdStats("location:get", "")
		if loc := DBGetLocation(user); loc == "" {
			json.NewEncoder(w).Encode(
				APIResponse{
					true,
					fmt.Sprint("User does not have a location"),
					"",
				},
			)
		} else {
			json.NewEncoder(w).Encode(
				APIResponse{
					false,
					"",
					loc,
				},
			)
		}
		return
	}

	if r.Method == "PUT" { // change location
		go CmdStats("location:put", "")
		var loc = vars["loc"]
		if err := DBSetLocation(user, loc); err != nil {
			json.NewEncoder(w).Encode(
				APIResponse{
					true,
					fmt.Sprintf(
						"Database error: %v \nPlease report this error to the developers",
						err.Error()),
					"",
				},
			)
			logError("unable to change location", err)
		} else {
			json.NewEncoder(w).Encode(
				APIResponse{
					false,
					"",
					"Location changed successfully",
				},
			)
			log.WithFields(log.Fields{
				"user":     user,
				"location": loc,
			}).Debug("location-change")
		}
	}
}

// BuyItem is the route for buying items
func BuyItem(w http.ResponseWriter, r *http.Request) {
	var item BuyItemRequest
	defer r.Body.Close()
	err := readAndUnmarshal(r.Body, &item)
	if err != nil {
		fmt.Println("Error reading and unmarshaling request:", err.Error())
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error reading and unmarshaling request:", err.Error()),
				UserItems{},
			},
		)
		return
	}

	user := mux.Vars(r)["userID"]

	if DBCheckBlacklist(user) {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("User blacklisted"),
				"",
			},
		)
		return
	}

	DBGetInventory(user)
	err = DBEditItemTier(user, item.Category, fmt.Sprintf("%v", item.Current))
	if err != nil {
		logError("unable to edit item tier", err)
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error editing item tier:", err.Error()),
				UserItems{},
			},
		)
		return
	}
	err = DBEditOwnedItems(user, item.Category, item.Owned)
	if err != nil {
		logError("unable to edit owned items", err)
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error editing item tier:", err.Error()),
				UserItems{},
			},
		)
		return
	}

	json.NewEncoder(w).Encode(
		APIResponse{
			false,
			"",
			DBGetInventory(user),
		},
	)
	log.WithFields(log.Fields{
		"user":     user,
		"category": item.Category,
		"item":     fmt.Sprintf("%v", item.Current),
	}).Debug("item-bought")
}

// Blacklist blacklists a user from using fishy
func Blacklist(w http.ResponseWriter, r *http.Request) {
	DBBlackListUser(mux.Vars(r)["userID"])
	fmt.Fprint(w, ":ok_hand:")
}

// Unblacklist unblacklists a user from using fishy
func Unblacklist(w http.ResponseWriter, r *http.Request) {
	DBUnblackListUser(mux.Vars(r)["userID"])
	fmt.Fprint(w, "sad to see you go...")
}

// StartGatherBait starts the timeout for gathering bait
func StartGatherBait(w http.ResponseWriter, r *http.Request) {
	DBStartGatherBait(mux.Vars(r)["userID"])
	fmt.Fprint(w, ":ok_hand: you decide to spend the next 6 hours filling up your bait box with bait")
	log.WithFields(log.Fields{
		"user": mux.Vars(r)["userID"],
	}).Debug("user-gather-bait")
}

// CheckGatherBait checks to see if a user is still gathering bait and will return the time remaining
func CheckGatherBait(w http.ResponseWriter, r *http.Request) {

}

// GetLeaderboard gets a specified leaderboard
func GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	var data LeaderboardRequest
	var s []redis.Z
	var scores []LeaderboardUser
	var err error
	if err := readAndUnmarshal(r.Body, &data); err != nil {
		respondError(w, true,
			fmt.Sprintf(
				"Request error: %v",
				err,
			),
		)
		return
	}
	if data.Global {
		s, err = DBGetGlobalScorePage(data.Page)
		if err != nil {
			respondError(w, true,
				fmt.Sprintf(
					"Could not retrieve scores: %v",
					err.Error(),
				),
			)
			return
		}
	} else {
		s, err = DBGetGuildScorePage(data.GuildID, data.Page)
		if err != nil {
			respondError(w, true,
				fmt.Sprintf(
					"Could not retrieve scores: %v",
					err.Error(),
				),
			)
			return
		}
	}
	for _, e := range s {
		scores = append(scores, LeaderboardUser{e.Score, e.Member})
	}

	l, err := LeaderboardTemp(scores, data.Global, data.User, data.GuildID, data.GuildName)
	if err != nil {
		respondError(w, true,
			fmt.Sprintf(
				"Could not retrieve scores: %v",
				err.Error(),
			),
		)
		return
	}
	//fmt.Fprint(w, l)
	respond(w, l)
}

//
func CheckTime(w http.ResponseWriter, r *http.Request) {
	var morning, night bool

	if CurrentTime.After(Morning1) && CurrentTime.Before(Morning2) {
		morning = true
	}
	if CurrentTime.After(Night1) || CurrentTime.Before(Night2) {
		night = true
	}

	respond(w,
		TimeData{
			CurrentTime.Format(time.Kitchen),
			morning,
			night,
		},
	)
}

//
func RandTrash(w http.ResponseWriter, r *http.Request) {
	respond(w, "you caught "+randomTrash())
}

//
func CommandStats(w http.ResponseWriter, r *http.Request) {
	stats, err := DBGetCmdStats("fish") // todo: other commands
	if err != nil {
		respondError(w, true,
			fmt.Sprintf(
				"Error retrieving command stats: %v",
				err,
			),
		)
		return
	}
	respond(w, stats)
}

//
func RandFish(w http.ResponseWriter, r *http.Request) {
	respond(w,
		makeEmbedFish(
			getFish(5, "ocean"),
			"hey idiot",
			UserLocDensity{},
		),
	)
}

//
func BaitInvGet(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	respond(w,
		map[string]interface{}{
			"maxBait":          DBGetBaitCapacity(user),
			"currentBaitCount": DBGetBaitUsage(user),
			"bait":             DBGetBaitInv(user),
			"currentTier":      DBGetCurrentBaitTier(user),
			"baitbox":          DBGetInventory(user).BaitBox.Current,
		},
	)
}

//
func BaitInvPost(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	var bait BaitRequest
	err := readAndUnmarshal(r.Body, &bait)
	if err != nil {
		respondError(w, true,
			fmt.Sprintf("Error unmarshaling request: %s", err.Error()),
		)
		return
	}
	before, amt, err := DBAddBait(user, bait.Tier, bait.Amount)
	if err != nil {
		respondError(w, true,
			fmt.Sprintf("Error adding bait: %s", err.Error()),
		)
		return
	}
	respond(w,
		map[string]interface{}{
			"new":   amt,
			"added": amt - int64(before),
		},
	)
}

//
func EquippedBaitGet(w http.ResponseWriter, r *http.Request) {
	respond(w,
		map[string]interface{}{
			"tier": DBGetCurrentBaitTier(mux.Vars(r)["userID"]),
		},
	)
}

//
func EquippedBaitPost(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	err := readAndUnmarshal(r.Body, &req)
	if err != nil {
		fmt.Println("Error unmarshaling request data " + err.Error())
		respondError(w, true, err.Error())
		return
	}
	err = DBSetCurrentBaitTier(mux.Vars(r)["userID"], req["tier"].(float64))
	if err != nil {
		fmt.Println("Error setting current bait " + err.Error())
		respondError(w, true, err.Error())
		return
	}
	respond(w, fmt.Sprintf("Successfully set current bait tier to %v", req["tier"].(float64)))
}

//
func SellFish(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	worth := DBSellFish(user)
	respond(w,
		fmt.Sprintf(
			"You redeemed %s fish, %s legendaries, and %s garbage for %s :yen:",
			worth["fish"], worth["legendaries"], worth["garbage"], worth["worth"],
		),
	)
	log.WithFields(log.Fields{
		"user":        user,
		"worth":       worth["worth"],
		"fish":        worth["fish"],
		"legendaries": worth["legendaries"],
		"garbage":     worth["garbage"],
	}).Debug("user-sell-fish")
}

//
func Stats(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	guild := mux.Vars(r)["guildID"]
	globalStats := DBGetGlobalStats(user)
	guildStats := DBGetGuildStats(user, guild)
	respond(w,
		map[string]interface{}{
			"guild":  guildStats,
			"global": globalStats,
		},
	)
	log.WithFields(log.Fields{
		"user":  user,
		"guild": guild,
	}).Debug("user-stats")
}
