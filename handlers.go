package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/iopred/discordgo"
)

// Index responds with Hello World so it can easily be tested if the API is running
func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello world\n")
	DBCmdStats("fishy")
}

// Fish is the main route for t!fishy
func Fish(w http.ResponseWriter, r *http.Request) {
	var msg *discordgo.Message
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("Error reading POST body " + err.Error())
		return
	}
	err = json.Unmarshal(data, &msg)
	if err != nil {
		fmt.Println("Error unmarshaling json " + err.Error())
		return
	}
	fmt.Fprint(w, ":fishing_pole_and_fish:  |  "+msg.Author.Username+", you caught: :fish:! You paid :yen: 10 for casting.")
}
