package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/iopred/discordgo"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// Index responds with Hello World so it can easily be tested if the API is running
func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello world\n")
}

// Fishy is the main route for t!fishy
func Fishy(w http.ResponseWriter, r *http.Request) {
	var msg *discordgo.Message
	defer r.Body.Close()
	if err := readAndUnmarshal(r.Body, &msg); err != nil {
		respondError(w, fmt.Sprintf(
			"Error reading and unmarshaling request\n%v", err.Error()))
		return
	}
	go CmdStats("fishy", msg.ID)
	go DBTrackUser(msg.Author)
	if DBCheckBlacklist(msg.Author.ID) {
		respondError(w, fmt.Sprintf(
			":x: | User %v#%v has been blacklisted from fishing.", msg.Author.Username, msg.Author.Discriminator))
		return
	}
	if gathering, timeLeft := DBCheckGatherBait(msg.Author.ID); gathering {
		respondError(w, fmt.Sprintf(
			":x: | You are currently gathering bait. Please wait %v for you to finish.", timeLeft.String()))
		return
	}
	if rl, timeLeft := DBCheckRateLimit("fishy", msg.Author.ID); rl {
		respondError(w, fmt.Sprintf(
			"Please wait %v before fishing again!", timeLeft.String()))
		return
	}
	fmt.Println(msg.Author.Username)
	//inv := DBGetInventory(msg.Author.ID)
	noinv := DBCheckMissingInventory(msg.Author.ID)
	if len(noinv) > 0 {
		sort.Strings(noinv)

		if i := sort.SearchStrings(noinv, "rod"); i < len(noinv) && noinv[i] == "rod" {
			DBIncInvEE(msg.Author.ID)
			a := DBGetInvEE(msg.Author.ID)
			num := math.Floor(float64(a / 10))
			respondError(w, Secrets.InvEE[int(num)])
			if num == float64(len(Secrets.InvEE))-1 {
				DBEditItemTier(msg.Author.ID, "rod", "1")
				DBEditItemTier(msg.Author.ID, "hook", "1")
			}
			return
		}
		if i := sort.SearchStrings(noinv, "hook"); i < len(noinv) && noinv[i] == "hook" {
			respondError(w, fmt.Sprint(
				"You cast your line but it just sits on the surface\n"+
					"*Something inside of you thinks that fish won't bite without a hook...*"))
			return
		}
		respondError(w, fmt.Sprintf(
			"You do not own the correct equipment for fishing\n"+
				"Please buy the following items: %v", strings.Join(noinv, ", ")))
		return
	}

	if amt, err := DBGetCurrentBaitAmt(msg.Author.ID); err != nil {
		respondError(w, fmt.Sprintf(
			"There was an error"))
		logError("Error converting current bait tier", err)
		return
	} else {
		if amt < 1 {
			respondError(w, fmt.Sprintf(
				"You do not own any bait of your currently equipped tier. Please buy more bait or switch tiers."))
			return
		}
	}

	bite := DBGetBiteRate(msg.Author.ID)
	catch, err := DBGetCatchRate(msg.Author.ID)
	if err != nil {
		respondError(w, err.Error())
	}
	fish, err := DBGetFishRate(msg.Author.ID)
	if err != nil {
		respondError(w, err.Error())
	}
	loc := DBGetLocation(msg.Author.ID)
	//density, _ := DBGetSetLocDensity(loc, msg.Author.ID)
	// score := DBGetGlobalScore(msg.Author.ID)
	fc, e := fishCatch(bite, catch, fish)

	if fc {
		if e == "garbage" {
			go DBAddFishToInv(msg.Author.ID, "garbage", 5)
			respond(w, makeEmbedTrash(msg.Author.Username, loc, randomTrash()))

			// fmt.Sprintf(
			// 	"%v fishing in %v\n"+
			// 		"you caught %v", msg.Author.Username, loc, randomTrash()))
		}
		if e == "fish" {
			level := expToTier(DBGetGlobalScore(msg.Author.ID))
			f := getFish(level, loc)
			err := DBAddFishToInv(msg.Author.ID, "fish", f.Price)
			if err != nil {
				respondError(w, "Your fish inventory is full and you cannot carry any more. You are forced to throw the fish back.")
			} else {
				go DBGiveGlobalScore(msg.Author.ID, 1)
				go DBLoseBait(msg.Author.ID)
				respond(w, makeEmbedFish(f, msg.Author.Username))
			}

			// fmt.Sprintf(
			// 	"%v fishing in %v\n"+
			// 		"you caught a tier %v %v. It is %vcm long and worth %v.\n%s\n%s", msg.Author.Username, loc, f.Tier, f.Name, f.Size, f.Price, f.Pun, f.URL))
		}
	} else {
		respond(w, makeEmbedFail(msg.Author.Username, loc, failed(e, msg.Author.ID)))

		// fmt.Sprintf(
		// 	"%v fishing in %v\n"+
		// 		"%v", msg.Author.Username, loc, failed(e)))
	}

	go DBSetRateLimit("fishy", msg.Author.ID, FishyTimeout)
}

func makeEmbedFail(user, location, fail string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		//Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: "https://cdn.discordapp.com/attachments/288505799905378304/332261752777736193/Can.png"},
		Color:       0xFF0000,
		Title:       fmt.Sprintf("%s, you were unable to catch anything", user),
		Description: fail,
	}
}

func makeEmbedTrash(user, location, trash string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: "https://cdn.discordapp.com/attachments/288505799905378304/332261752777736193/Can.png"},
		Color:       0xffffff,
		Title:       fmt.Sprintf("%s, you fished up some trash in the %s", user, location),
		Description: fmt.Sprintf("It's %s", trash),
	}
}

func makeEmbedFish(fish InvFish, user string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: fish.URL},
		Color:       tierToEmbedColor(fish.Tier),
		Title:       fmt.Sprintf("%s, you caught a %s in the %s", user, fish.Name, fish.Location),
		Description: fish.Pun,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Length", Value: fmt.Sprintf("%.2fcm", fish.Size), Inline: false},
			&discordgo.MessageEmbedField{Name: "Price", Value: fmt.Sprintf("%.0f¥", fish.Price), Inline: false},
		},
	}
}

func tierToEmbedColor(tier int) int {
	switch tier {
	case 1:
		return 0xe2e2e2
	case 2:
		return 0x80b3f4
	case 3:
		return 0x80fe80
	case 4:
		return 0xa96aed
	case 5:
		return 0xffd000
	}
	return 0x000000
}

// Inventory is the main route for getting a user's item inventory
func Inventory(w http.ResponseWriter, r *http.Request) {
	//go CmdStats("inventory:get", "")
	user := mux.Vars(r)["userID"]

	respond(w,
		map[string]interface{}{
			"items":    DBGetInventory(user),
			"fish":     DBGetFishInv(user),
			"maxFish":  DBGetInvCapacity(user),
			"maxBait":  DBGetBaitCapacity(user),
			"userTier": expToTier(DBGetGlobalScore(user))})
}

// Location is the main route for getting and changing or getting a user's location
func Location(w http.ResponseWriter, r *http.Request) {
	var vars = mux.Vars(r)
	var user = vars["userID"]

	if DBCheckBlacklist(user) {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("User blacklisted"),
				""})
		return
	}

	if r.Method == "GET" { // get location
		go CmdStats("location:get", "")
		if loc := DBGetLocation(user); loc == "" {
			json.NewEncoder(w).Encode(
				APIResponse{
					true,
					fmt.Sprint("User does not have a location"),
					""})
		} else {
			json.NewEncoder(w).Encode(
				APIResponse{
					false,
					"",
					loc})
		}
		return
	}

	if r.Method == "PUT" { // change location
		go CmdStats("location:put", "")
		var loc = vars["loc"]
		if err := DBSetLocation(user, loc); err != nil {
			json.NewEncoder(w).Encode(
				APIResponse{
					true,
					fmt.Sprintf("Database error: %v \n"+
						"Please report this error to the developers", err.Error()),
					""})
		} else {
			json.NewEncoder(w).Encode(
				APIResponse{
					false,
					"",
					"Location changed successfully"})
		}
	}
}

// BuyItem is the route for buying items
func BuyItem(w http.ResponseWriter, r *http.Request) {
	var item BuyItemRequest
	go CmdStats("item", "")
	defer r.Body.Close()
	err := readAndUnmarshal(r.Body, &item)
	if err != nil {
		fmt.Println("Error reading and unmarshaling request:", err.Error())
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error reading and unmarshaling request:", err.Error()),
				UserItems{}})
		return
	}

	user := mux.Vars(r)["userID"]

	if DBCheckBlacklist(user) {
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("User blacklisted"),
				""})
		return
	}

	DBGetInventory(user)
	err = DBEditItemTier(user, item.Item, item.Tier)
	if err != nil {
		fmt.Println("error editing item tier", err.Error())
		json.NewEncoder(w).Encode(
			APIResponse{
				true,
				fmt.Sprint("Error editing item tier:", err.Error()),
				UserItems{}})
		return
	}
	json.NewEncoder(w).Encode(
		APIResponse{
			false,
			"",
			DBGetInventory(user)})
}

// Blacklist blacklists a user from using fishy
func Blacklist(w http.ResponseWriter, r *http.Request) {
	DBBlackListUser(mux.Vars(r)["userID"])
	fmt.Fprint(w, ":ok_hand:")
}

// Unblacklist unblacklists a user from using fishy
func Unblacklist(w http.ResponseWriter, r *http.Request) {
	DBUnblackListUser(mux.Vars(r)["userID"])
	fmt.Fprint(w, "sad to see you go...")
}

// StartGatherBait starts the timeout for gathering bait
func StartGatherBait(w http.ResponseWriter, r *http.Request) {
	DBStartGatherBait(mux.Vars(r)["userID"])
	fmt.Fprint(w, ":ok_hand: you decide to spend the next 6 hours filling up your bait box with bait")
}

// CheckGatherBait checks to see if a user is still gathering bait and will return the time remaining
func CheckGatherBait(w http.ResponseWriter, r *http.Request) {

}

// GetLeaderboard gets a specified leaderboard
func GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	var data LeaderboardRequest
	var s []redis.Z
	var scores []LeaderboardUser
	var err error
	if err := readAndUnmarshal(r.Body, &data); err != nil {
		respondError(w, fmt.Sprintf(
			"Request error: %v", err))
		return
	}
	if data.Global {
		s, err = DBGetGlobalScorePage(data.Page)
		if err != nil {
			respondError(w, fmt.Sprintf("Could not retrieve scores: %v", err.Error()))
			return
		}
	} else {
		s, err = DBGetGuildScorePage(data.GuildID, data.Page)
		if err != nil {
			respondError(w, fmt.Sprintf("Could not retrieve scores: %v", err.Error()))
			return
		}
	}
	for _, e := range s {
		scores = append(scores, LeaderboardUser{e.Score, e.Member})
	}

	l, err := LeaderboardTemp(scores, data.Global, data.User, data.GuildID, data.GuildName)
	if err != nil {
		respondError(w, fmt.Sprintf("Could not retrieve scores: %v", err.Error()))
		return
	}
	//fmt.Fprint(w, l)
	respond(w, l)
}

//
func CheckTime(w http.ResponseWriter, r *http.Request) {
	var morning, night bool

	if CurrentTime.After(Morning1) && CurrentTime.Before(Morning2) {
		morning = true
	}
	if CurrentTime.After(Night1) || CurrentTime.Before(Night2) {
		night = true
	}

	respond(w, TimeData{CurrentTime.Format(time.Kitchen), morning, night})
}

//
func RandTrash(w http.ResponseWriter, r *http.Request) {
	respond(w, "you caught "+randomTrash())
}

//
func CommandStats(w http.ResponseWriter, r *http.Request) {
	stats, err := DBGetCmdStats("fish") // todo: other commands
	if err != nil {
		respondError(w, fmt.Sprintf(
			"Error retrieving command stats: %v", err))
		return
	}
	respond(w, stats)
}

//
func RandFish(w http.ResponseWriter, r *http.Request) {
	respond(w, makeEmbedFish(getFish(5, "ocean"), "hey idiot"))
}

//
func BaitInvGet(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	respond(w,
		map[string]interface{}{
			"maxBait":          DBGetBaitCapacity(user),
			"currentBaitCount": DBGetBaitUsage(user),
			"bait":             DBGetBaitInv(user),
			"currentTier":      DBGetCurrentBaitTier(user)})
}

//
func BaitInvPost(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	var bait BaitRequest
	err := readAndUnmarshal(r.Body, &bait)
	if err != nil {
		respondError(w,
			fmt.Sprintf("Error unmarshaling request: %s", err.Error()))
		return
	}
	amt, err := DBAddBait(user, bait.Tier, bait.Amount)
	if err != nil {
		respondError(w,
			fmt.Sprintf("Error adding bait: %s", err.Error()))
		return
	}
	respond(w,
		map[string]interface{}{
			"new": amt})
}

//
func EquippedBaitGet(w http.ResponseWriter, r *http.Request) {
	respond(w,
		map[string]interface{}{
			"tier": DBGetCurrentBaitTier(mux.Vars(r)["userID"])})
}

//
func EquippedBaitPost(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	err := readAndUnmarshal(r.Body, &req)
	if err != nil {
		fmt.Println("Error unmarshaling request data " + err.Error())
		respondError(w, err.Error())
		return
	}
	err = DBSetCurrentBaitTier(mux.Vars(r)["userID"], req["tier"].(float64))
	if err != nil {
		fmt.Println("Error setting current bait " + err.Error())
		respondError(w, err.Error())
		return
	}
	respond(w, fmt.Sprintf("Successfully set current bait tier to %v", req["tier"].(float64)))
}

//
func FishInventory(w http.ResponseWriter, r *http.Request) {

}

//
func SellFish(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["userID"]
	worth := DBSellFish(user)
	respond(w, fmt.Sprintf("You redeemed %s fish, %s legendaries, and %s garbage for %s credits", worth["fish"], worth["legendaries"], worth["garbage"], worth["worth"]))
}

func respond(w http.ResponseWriter, data interface{}) {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	e.Encode(
		APIResponse{
			false,
			"",
			data})
}

func respondError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(
		APIResponse{
			true,
			err,
			""})
}

func fishCatch(bite, catch, fish float32) (bool, string) {
	r1 := rand.Float32()
	r2 := rand.Float32()
	r3 := rand.Float32()
	fmt.Println(r1, bite)
	fmt.Println(r2, catch)
	fmt.Println(r3, fish)

	if r1 <= bite {
		if r2 <= catch {
			if r3 <= fish {
				return true, "fish"
			}
			return true, "garbage"
		}
		return false, "catch"
	}
	return false, "bite"
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

func failed(e, uID string) string {
	if e == "catch" {
		go DBLoseBait(uID)
		return "a fish bit but you were unable to wrangle it in"
	}
	if e == "bite" {
		return "you couldn't get a fish to bite"
	}
	return ""
}

func randomTrash() string {
	r := rand.Intn(len(Trash.Regular.Text) - 1)
	return Trash.Regular.Text[r]
}

var t1 = 50
var t2 = 29
var t3 = 15
var t4 = 5
var t5 = 1
var t1Total = t1
var t2Total = t1Total + t2
var t3Total = t2Total + t3
var t4Total = t3Total + t4
var t5Total = t4Total + t5

func getFish(tier int, location string) InvFish {
	_tier := selectTier(tier)
	base := Fish.Location.Ocean
	switch location {
	case "lake":
		base = Fish.Location.Lake
	case "river":
		base = Fish.Location.River
	}
	fish := base[_tier-1].Fish
	// fish number
	r1 := rand.Intn(len(fish) - 1)
	_fish := fish[r1]
	// fish len
	r := float64(rand.Intn(_fish.Size[1]-_fish.Size[0]) + _fish.Size[0])
	r += rand.Float64()
	sellPrice := getFishPrice(_tier, float64(_fish.Size[0]), float64(_fish.Size[1]), float64(r))
	return InvFish{location, _fish.Name, sellPrice, r, _tier, _fish.Pun, _fish.Image}
}

func getFishPrice(tier int, min, max, l float64) float64 {
	var ratio, price float64
	switch tier {
	case 1:
		ratio = (l - min) / (max - min)
		price = ((Fish.Prices[0][1] - Fish.Prices[0][0]) * ratio) + Fish.Prices[0][0]
	case 2:
		ratio = (l - min) / (max - min)
		price = ((Fish.Prices[1][1] - Fish.Prices[1][0]) * ratio) + Fish.Prices[1][0]
	case 3:
		ratio = (l - min) / (max - min)
		price = ((Fish.Prices[2][1] - Fish.Prices[2][0]) * ratio) + Fish.Prices[2][0]
	case 4:
		ratio = (l - min) / (max - min)
		price = ((Fish.Prices[3][1] - Fish.Prices[3][0]) * ratio) + Fish.Prices[3][0]
	case 5:
		ratio = (l - min) / (max - min)
		price = ((Fish.Prices[4][1] - Fish.Prices[4][0]) * ratio) + Fish.Prices[4][0]
	default:
		logError("Error getting fish price", errors.New("Unknown tier in price calculation"))
		return price
	}

	return math.Floor(price)
}

func selectTier(userTier int) int {
	switch userTier {
	case 1:
		return 1

	case 2:
		sel := rand.Intn(t2Total)
		switch {
		case sel <= t1Total:
			return 1
		default:
			return 2
		}

	case 3:
		sel := rand.Intn(t3Total)
		switch {
		case sel <= t1Total:
			return 1
		case sel <= t2Total:
			return 2
		default:
			return 3
		}

	case 4:
		sel := rand.Intn(t4Total)
		switch {
		case sel <= t1Total:
			return 1
		case sel <= t2Total:
			return 2
		case sel <= t3Total:
			return 3
		default:
			return 4
		}

	default:
		sel := rand.Intn(t5Total)
		switch {
		case sel <= t1Total:
			return 1
		case sel <= t2Total:
			return 2
		case sel <= t3Total:
			return 3
		case sel <= t4Total:
			return 4
		default:
			return 5
		}
	}
}

func expToTier(e float64) int {
	switch {
	case e >= 1000:
		return 5
	case e >= 500:
		return 4
	case e >= 250:
		return 3
	case e >= 100:
		return 2
	case e >= 0:
		return 1
	}
	return 1
}
