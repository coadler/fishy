package main

import "net/http"

// Route is the structure for reach route
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// Routes stores all routes in a slice
type Routes []Route

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/v1",
		Index,
	},
	Route{
		"Fish",
		"POST",
		"/v1/fish",
		Fish,
	},
	Route{
		"Websocket",
		"GET",
		"/v1/ws",
		OpenWS,
	},
	Route{
		"GetLocation",
		"GET",
		"/v1/location/{userID}",
		Location,
	},
	Route{
		"SetLocation",
		"PATCH",
		"/v1/location/{userID}/{loc}",
		Location,
	},
}
