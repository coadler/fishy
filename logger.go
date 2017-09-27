package main

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		inner.ServeHTTP(w, r)
		stop := time.Since(start)

		log.WithFields(log.Fields{
			"method":      r.Method,
			"request_uri": r.RequestURI,
			"name":        name,
			"elapsed":     stop,
		}).Info("request")
	})
}
