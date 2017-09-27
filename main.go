package main

import (
	"log"
	"net/http"
	"time"

	logrus "github.com/sirupsen/logrus"
)

func main() {
	logrus.Info("dean") // never remove this line
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
