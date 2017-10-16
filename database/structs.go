package database

// InvFish holds the JSON structure for a singular fish
type InvFish struct {
	Location string  `json:"location"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Size     float64 `json:"size"`
	Tier     int     `json:"tier"`
	Pun      string  `json:"pun"`
	URL      string  `json:"url"`
}

//
type FishInv struct {
	Fish        int `json:"fish"`
	Garbage     int `json:"garbage"`
	Legendaries int `json:"legendaries"`
	Worth       int `json:"worth"`
}

// UserItems holds the JSON structure for a users items
// type UserItems struct {
// 	Bait    int `json:"bait"`
// 	Rod     int `json:"rod"`
// 	Hook    int `json:"hook"`
// 	Vehicle int `json:"vehicle"`
// 	BaitBox int `json:"baitbox"`
// }

// UserItems stores all the item categories for a specific user
type UserItems struct {
	Bait    UserItem `json:"bait"`
	Rod     UserItem `json:"rod"`
	Hook    UserItem `json:"hook"`
	Vehicle UserItem `json:"vehicle"`
	BaitBox UserItem `json:"bait_box"`
}

// UserItem stores the data for each item category
type UserItem struct {
	Current int   `json:"current"`
	Owned   []int `json:"owned"`
}

// BaitInv stores the bait tier amounts for a specific user
type BaitInv struct {
	T1 int `json:"t1"`
	T2 int `json:"t2"`
	T3 int `json:"t3"`
	T4 int `json:"t4"`
	T5 int `json:"t5"`
}

// BaitRequest stores the data for BaitInvPost
type BaitRequest struct {
	Tier   int `json:"tier"`
	Amount int `json:"amount"`
}

// UserLocDensity stores the location density for each user
type UserLocDensity struct {
	Lake  int `json:"lake"`
	River int `json:"river"`
	Ocean int `json:"ocean"`
}

// LocationResponse holds the JSON structure for the location endpoint
type LocationResponse struct {
	Location string `json:"location"`
	Error    bool   `json:"error"`
}

// BuyItemRequest holds the request structure for buying an item
type BuyItemRequest struct {
	Category string `json:"category"`
	Current  int    `json:"current"`
	Owned    []int  `json:"owned"`
}

// APIResponse is a standard API response
type APIResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// LeaderboardRequest stores the data for GetLeaderboard
type LeaderboardRequest struct {
	Global    bool   `json:"global"`
	Page      int    `json:"page"`
	User      string `json:"user"`
	GuildID   string `json:"guildid,omitempty"`
	GuildName string `json:"guildname,omitempty"`
}

//
type LeaderboardData struct {
	Scores    []LeaderboardUser
	Rank      int64
	Score     float64
	GuildName string
	Global    bool
}

//
type LeaderboardUser struct {
	Score  float64
	Member interface{}
}

//
type TimeData struct {
	Time    string `json:"time"`
	Morning bool   `json:"morning"`
	Night   bool   `json:"night"`
}

//
type UserStats struct {
	Garbage   int     `json:"garbage"`
	Fish      int     `json:"fish"`
	AvgLength float64 `json:"avglength"`
	Casts     int     `json:"casts"`
}

//
type CommandStatData struct {
	Hourly int `json:"hourly"`
	Daily  int `json:"daily"`
	Total  int `json:"total"`
}
