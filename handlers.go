package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

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
	err := readAndUnmarshal(r.Body, &msg)
	if err != nil {
		fmt.Println("Error reading and unmarshaling request", err.Error())
		return
	}
	if DBCheckBlacklist(msg.Author.ID) {
		fmt.Fprint(w, "you're blacklisted muahahahahhahah")
		return
	}
	rl, timeLeft := DBCheckRateLimit("fishy", msg.Author.ID)
	if rl {
		fmt.Fprint(w, "Please wait ", timeLeft.String(), " before fishing again!")
		return
	}
	inv := DBGetInventory(msg.Author.ID)
	noinv := DBCheckInventory(msg.Author.ID)
	if len(noinv) > 0 {
		fmt.Fprintf(w,
			"You do not own the correct equipment for fishing\n"+
				"Please buy the following items: %v", noinv)
		return
	}

	bite := DBGetBiteRate(msg.Author.ID)
	loc := DBGetLocation(msg.Author.ID)
	density, err := DBGetSetLocDensity(loc, msg.Author.ID)
	score := DBGetGlobalScore(msg.Author.ID)
	go DBGiveGlobalScore(msg.Author.ID, 1)

	fmt.Fprintf(w,
		"%v fishing in %v \n"+
			"%+v \n"+
			"biterate: %v\n"+
			"exp: %v\n"+
			"own: %+v\n"+
			"have not bought: %v", msg.Author.Username, loc, density, bite, score, inv, noinv)

	go DBSetRateLimit("fishy", msg.Author.ID, 10*time.Second)
}

// Inventory is the main route for getting a user's item inventory
func Inventory(w http.ResponseWriter, r *http.Request) {
	go DBCmdStats("inventory:get")
	json.NewEncoder(w).Encode(DBGetInventory(mux.Vars(r)["userID"]))
}

// Location is the main route for changing or getting a user's location
func Location(w http.ResponseWriter, r *http.Request) {
	var respErr = false
	var vars = mux.Vars(r)
	var user = vars["userID"]

	if r.Method == "GET" { // get location
		go DBCmdStats("location:get")
		loc := DBGetLocation(user)
		if loc == "" {
			respErr = true
		}
		json.NewEncoder(w).Encode(LocationResponse{loc, respErr})
		return
	}

	if r.Method == "PUT" { // change location
		go DBCmdStats("location:put")
		var loc = vars["loc"]
		if err := DBSetLocation(user, loc); err != nil {
			fmt.Println("Error setting location", err.Error())
			respErr = true
		}
		json.NewEncoder(w).Encode(LocationResponse{loc, respErr})
		return
	}

}

// BuyItem is the route for buying items
func BuyItem(w http.ResponseWriter, r *http.Request) {
	var item BuyItemRequest
	go DBCmdStats("item")
	defer r.Body.Close()
	err := readAndUnmarshal(r.Body, &item)
	if err != nil {
		fmt.Println("Error reading and unmarshaling request", err.Error())
		json.NewEncoder(w).Encode(BuyItemResponse{UserItems{}, true})
		return
	}

	user := mux.Vars(r)["userID"]
	DBGetInventory(user)
	err = DBEditItemTier(user, item.Item, item.Tier)
	if err != nil {
		fmt.Println("error editing item tier", err.Error())
		json.NewEncoder(w).Encode(BuyItemResponse{UserItems{}, true})
		return
	}
	json.NewEncoder(w).Encode(BuyItemResponse{DBGetInventory(user), false})
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
