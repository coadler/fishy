package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/iopred/discordgo"
)

// Index responds with Hello World so it can easily be tested if the API is running
func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello world\n")
}

// Fishy is the main route for t!fishy
func Fishy(w http.ResponseWriter, r *http.Request) {
	go DBCmdStats("fishy")
	var msg *discordgo.Message
	defer r.Body.Close()
	if err := readAndUnmarshal(r.Body, &msg); err != nil {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error reading and unmarshaling request"),
				""})
		return
	}
	if DBCheckBlacklist(msg.Author.ID) {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint(":x: | You have been blacklisted from using fishy"),
				""})
		return
	}
	if gathering, timeLeft := DBCheckGatherBait(msg.Author.ID); gathering {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprintf("You are currently gathering bait! Please wait %v to finish gathering your bait", timeLeft.String()),
				""})
		return
	}
	if rl, timeLeft := DBCheckRateLimit("fishy", msg.Author.ID); rl {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprintf("Please wait %v before fishing again!", timeLeft.String()),
				""})
		return
	}
	inv := DBGetInventory(msg.Author.ID)
	noinv := DBCheckMissingInventory(msg.Author.ID)
	if len(noinv) > 0 {
		fmt.Fprintf(w,
			"You do not own the correct equipment for fishing\n"+
				"Please buy the following items: %v", strings.Join(noinv, ", "))
		return
	}

	bite := DBGetBiteRate(msg.Author.ID)
	loc := DBGetLocation(msg.Author.ID)
	density, _ := DBGetSetLocDensity(loc, msg.Author.ID)
	score := DBGetGlobalScore(msg.Author.ID)
	go DBGiveGlobalScore(msg.Author.ID, 1)

	json.NewEncoder(w).Encode(
		APIResponse{
			false,
			"",
			fmt.Sprintf(
				"%v fishing in %v \n"+
					"%+v \n"+
					"biterate: %v\n"+
					"exp: %v\n"+
					"own: %+v", msg.Author.Username, loc, density, bite, score, inv)})

	go DBSetRateLimit("fishy", msg.Author.ID, FishyTimeout)
}

// Inventory is the main route for getting a user's item inventory
func Inventory(w http.ResponseWriter, r *http.Request) {
	go DBCmdStats("inventory:get")
	json.NewEncoder(w).Encode(
		APIResponse{
			false,
			"",
			DBGetInventory(mux.Vars(r)["userID"])})
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
				""})
		return
	}

	if r.Method == "GET" { // get location
		go DBCmdStats("location:get")
		if loc := DBGetLocation(user); loc == "" {
			json.NewEncoder(w).Encode(
				APIResponse{
					true,
					fmt.Sprint("User does not have a location"),
					""})
		} else {
			json.NewEncoder(w).Encode(
				APIResponse{
					false,
					"",
					loc})
		}
		return
	}

	if r.Method == "PUT" { // change location
		go DBCmdStats("location:put")
		var loc = vars["loc"]
		if err := DBSetLocation(user, loc); err != nil {
			json.NewEncoder(w).Encode(
				APIResponse{
					true,
					fmt.Sprintf("Database error: %v \n"+
						"Please report this error to the developers", err.Error()),
					""})
		} else {
			json.NewEncoder(w).Encode(
				APIResponse{
					false,
					"",
					"Location changed successfully"})
		}
	}
}

// BuyItem is the route for buying items
func BuyItem(w http.ResponseWriter, r *http.Request) {
	var item BuyItemRequest
	go DBCmdStats("item")
	defer r.Body.Close()
	err := readAndUnmarshal(r.Body, &item)
	if err != nil {
		fmt.Println("Error reading and unmarshaling request:", err.Error())
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error reading and unmarshaling request:", err.Error()),
				UserItems{}})
		return
	}

	user := mux.Vars(r)["userID"]

	if DBCheckBlacklist(user) {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("User blacklisted"),
				""})
		return
	}

	DBGetInventory(user)
	err = DBEditItemTier(user, item.Item, item.Tier)
	if err != nil {
		fmt.Println("error editing item tier", err.Error())
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error editing item tier:", err.Error()),
				UserItems{}})
		return
	}
	json.NewEncoder(w).Encode(
		APIResponse{
			false,
			"",
			DBGetInventory(user)})
}

// Blacklist blacklists a user from using fishy
func Blacklist(w http.ResponseWriter, r *http.Request) {
	DBBlackListUser(mux.Vars(r)["userID"])
	fmt.Fprintf(w, ":ok_hand:")
}

// Unblacklist unblacklists a user from using fishy
func Unblacklist(w http.ResponseWriter, r *http.Request) {
	DBUnblackListUser(mux.Vars(r)["userID"])
	fmt.Fprintf(w, "sad to see you go...")
}

// StartGatherBait starts the timeout for gathering bait
func StartGatherBait(w http.ResponseWriter, r *http.Request) {
	DBStartGatherBait(mux.Vars(r)["userID"])
	fmt.Fprint(w, ":ok_hand: you decide to spend the next 6 hours filling up your bait box with bait")
}

// CheckGatherBait checks to see if a user is still gathering bait and will return the time remaining
func CheckGatherBait(w http.ResponseWriter, r *http.Request) {

}

// GetLeaderboard gets a specified leaderboard
func GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	var data LeaderboardRequest
	var scores []redis.Z
	var err error
	if err := readAndUnmarshal(r.Body, &data); err != nil {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Request error"),
				""})
		return
	}
	if data.Global {
		scores, err = DBGetGlobalScorePage(data.Page)
		if err != nil {
			respondError(w, fmt.Sprint("Could not retrieve scores:", err.Error()))
			return
		}
	} else {
		scores, err = DBGetGuildScorePage(data.GuildID, data.Page)
		if err != nil {
			respondError(w, fmt.Sprint("Could not retrieve scores:", err.Error()))
			return
		}
	}
	l, err := LeaderboardTemp(scores, data.Global, data.User, data.GuildID, data.GuildName)
	if err != nil {
		respondError(w, fmt.Sprint("Could not retrieve scores:", err.Error()))
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

	respond(w, TimeData{CurrentTime.Format(time.Kitchen), morning, night})
}

func respond(w http.ResponseWriter, data interface{}) {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	e.Encode(
		APIResponse{
			false,
			"",
			data})
}

func respondError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(
		APIResponse{
			true,
			err,
			""})
}

func readAndUnmarshal(data io.Reader, fmt interface{}) error {
	body, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &fmt)
	if err != nil {
		return err
	}
	return nil
}
