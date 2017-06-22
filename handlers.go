package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"sort"

	"math"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/iopred/discordgo"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

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
	go DBTrackUser(msg.Author)
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
	//inv := DBGetInventory(msg.Author.ID)
	noinv := DBCheckMissingInventory(msg.Author.ID)
	if len(noinv) > 0 {
		sort.Strings(noinv)

		if i := sort.SearchStrings(noinv, "rod"); i < len(noinv) && noinv[i] == "rod" {
			DBIncInvEE(msg.Author.ID)
			a := DBGetInvEE(msg.Author.ID)
			num := math.Floor(float64(a / 10))
			respondError(w, Secrets.InvEE[int(num)])
			if num == float64(len(Secrets.InvEE))-1 {
				DBEditItemTier(msg.Author.ID, "rod", "1")
				DBEditItemTier(msg.Author.ID, "hook", "1")
			}
			return
		}
		if i := sort.SearchStrings(noinv, "hook"); i < len(noinv) && noinv[i] == "hook" {
			respondError(w, fmt.Sprint(
				"You cast your line but it just sits on the surface\n"+
					"*Something inside of you thinks that fish won't bite without a hook...*"))
			return
		}
		respondError(w, fmt.Sprintf(
			"You do not own the correct equipment for fishing\n"+
				"Please buy the following items: %v", strings.Join(noinv, ", ")))
		return
	}

	bite := DBGetBiteRate(msg.Author.ID)
	catch, err := DBGetCatchRate(msg.Author.ID)
	if err != nil {
		respondError(w, err.Error())
	}
	fish, err := DBGetFishRate(msg.Author.ID)
	if err != nil {
		respondError(w, err.Error())
	}
	loc := DBGetLocation(msg.Author.ID)
	// density, _ := DBGetSetLocDensity(loc, msg.Author.ID)
	// score := DBGetGlobalScore(msg.Author.ID)
	fc, e := fishCatch(bite, catch, fish)

	if fc {
		if e == "garbage" {
			r := rand.Intn(len(Fish.Trash.Regular) - 1)
			respond(w, fmt.Sprintf(
				"%v fishing in %v\n"+
					"you caught %v", msg.Author.Username, loc, Fish.Trash.Regular[r]))
		}
		if e == "fish" {
			go DBGiveGlobalScore(msg.Author.ID, 1)
			respond(w, fmt.Sprintf(
				"%v fishing in %v\n"+
					"you caught a fish!!! woooooooooooo", msg.Author.Username, loc))
		}
	} else {
		respond(w, fmt.Sprintf(
			"%v fishing in %v\n"+
				"you didnt catch anything\nfailed on: %v", msg.Author.Username, loc, e))
	}

	//go DBSetRateLimit("fishy", msg.Author.ID, FishyTimeout)
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
	var s []redis.Z
	var scores []LeaderboardUser
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
		s, err = DBGetGlobalScorePage(data.Page)
		if err != nil {
			respondError(w, fmt.Sprint("Could not retrieve scores:", err.Error()))
			return
		}
	} else {
		s, err = DBGetGuildScorePage(data.GuildID, data.Page)
		if err != nil {
			respondError(w, fmt.Sprint("Could not retrieve scores:", err.Error()))
			return
		}
	}
	for _, e := range s {
		scores = append(scores, LeaderboardUser{e.Score, e.Member})
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

func fishCatch(bite, catch, fish float32) (bool, string) {
	//var b, c, f bool
	r1 := rand.Float32()
	r2 := rand.Float32()
	r3 := rand.Float32()
	fmt.Println(r1, bite)
	fmt.Println(r2, catch)
	fmt.Println(r3, fish)

	if r1 <= bite {
		if r2 <= catch {
			if r3 <= fish {
				return true, "fish"
			}
			return true, "garbage"
		}
		return false, "catch"
	}
	return false, "bite"
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
