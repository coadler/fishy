package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan CreditRequest)

// upgrader updates Get requests to WS connections
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// CreditRequest is the structure for requesting a credit change
type CreditRequest struct {
	UserID string `json:"userid"`
	Amount int    `json:"amount"`
}

// CreditResponse is the structure for responding to a credit change
type CreditResponse struct {
	UserID string `json:"userid"`
	Amount int    `json:"amount"`
	Result string `json:"result"`
}

// OpenWS receives a websocket request and opens a websocket
func OpenWS(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial request to a websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure to close the connection when the function returns
	defer ws.Close()

	// Register a new client
	clients[ws] = true

	// for {
	var msg CreditRequest
	// 	// Read in a new message as JSON and map it to a Message object
	// 	err := ws.ReadJSON(&msg)
	// 	if err != nil {
	// 		log.Printf("error: %v", err)
	// 		delete(clients, ws)
	// 		break
	// 	}
	// 	// Send the newly received message to the broadcast channel
	// 	broadcast <- msg
	// }\
	for {
		err := ws.ReadJSON(&msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		UserID := msg.UserID
		Amount := strconv.Itoa(msg.Amount)
		if err = ws.WriteMessage(websocket.TextMessage, []byte("Success! UserID: "+UserID+" Amount: "+Amount)); err != nil {
			fmt.Println(err)
			return
		}
	}
}

func handleMessages() {
	for {
		// Grab the next message from the broadcast channel
		msg := <-broadcast
		// Send it out to every client that is currently connected
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
