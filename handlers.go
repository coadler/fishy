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
	rl, timeLeft := DBCheckRateLimit("fishy", msg.Author.ID)
	if rl {
		fmt.Fprint(w, "Please wait ", timeLeft.String(), " before fishing again!")
		return
	}

	bite := DBGetBiteRate(msg.Author.ID)
	loc := DBGetLocation(msg.Author.ID)
	density, err := DBGetSetLocDensity(loc, msg.Author.ID)
	exp := DBGetGlobalExp(msg.Author.ID)
	go DBGiveGlobalExp(msg.Author.ID, 1)

	fmt.Fprintf(w,
		"%v fishing in %v \n"+
			"%+v \n"+
			"biterate: %v\n"+
			"exp: %v", msg.Author.Username, loc, density, bite, exp)

	go DBSetRateLimit("fishy", msg.Author.ID, 10*time.Second)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
}

// Inventory is the main route for getting a user's item inventory
func Inventory(w http.ResponseWriter, r *http.Request) {
	go DBCmdStats("inventory")
	var vars = mux.Vars(r)
	var user = vars["userID"]

	json.NewEncoder(w).Encode(DBGetInventory(user))
}

// Location is the main route for changing or getting a user's location
func Location(w http.ResponseWriter, r *http.Request) {
	go DBCmdStats("location")
	var respErr = false
	var vars = mux.Vars(r)
	var user = vars["userID"]

	if r.Method == "GET" { // get location
		loc := DBGetLocation(user)
		if loc == "" {
			respErr = true
		}
		json.NewEncoder(w).Encode(LocationResponse{loc, respErr})
		return
	}

	if r.Method == "PATCH" { // change location
		var loc = vars["loc"]
		if err := DBSetLocation(user, loc); err != nil {
			fmt.Println("Error setting location", err.Error())
			respErr = true
		}
		json.NewEncoder(w).Encode(LocationResponse{loc, respErr})
		return
	}

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
