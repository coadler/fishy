package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	router := NewRouter()
	t := time.Tick(10 * time.Second)

	go func() {
		for {
			<-t
			CurrentTime = time.Now().UTC()
		}
	}()

	log.Fatal(http.ListenAndServe(":8080", router))
}
