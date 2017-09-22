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
		Fishy,
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
		"PUT",
		"/v1/location/{userID}/{loc}",
		Location,
	},
	Route{
		"GetInventory",
		"GET",
		"/v1/inventory/{userID}",
		Inventory,
	},
	Route{
		"SetItem",
		"POST",
		"/v1/inventory/{userID}",
		BuyItem,
	},
	Route{
		"Blacklist",
		"GET",
		"/v1/blacklist/{userID}",
		Blacklist,
	},
	Route{
		"Unblacklist",
		"DELETE",
		"/v1/blacklist/{userID}",
		Unblacklist,
	},
	Route{
		"Gather bait",
		"POST",
		"/v1/gather/{userID}",
		StartGatherBait,
	},
	Route{
		"Gather bait",
		"GET",
		"/v1/gather/{userID}",
		CheckGatherBait,
	},
	Route{
		"Leaderboard",
		"POST",
		"/v1/leaderboard",
		GetLeaderboard,
	},
	Route{
		"Time",
		"GET",
		"/v1/time",
		CheckTime,
	},
	Route{
		"Trash",
		"GET",
		"/v1/trash",
		RandTrash,
	},
	Route{
		"Stats",
		"GET",
		"/v1/stats",
		CommandStats,
	},
	Route{
		"RFish",
		"GET",
		"/v1/rfish",
		RandFish,
	},
	Route{
		"BaitInv",
		"GET",
		"/v1/bait/{userID}",
		BaitInvGet,
	},
	Route{
		"BaitInv",
		"POST",
		"/v1/bait/{userID}",
		BaitInvPost,
	},
	Route{
		"CurrentBait",
		"GET",
		"/v1/bait/{userID}/current",
		EquippedBaitGet,
	},
	Route{
		"CurrentBait",
		"POST",
		"/v1/bait/{userID}/current",
		EquippedBaitPost,
	},
	Route{
		"SellFish",
		"Get",
		"/v1/inventory/sell/{userID}",
		SellFish,
	},
}
