package routes

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	pRand "math/rand"
	"sort"
	"strings"

	"github.com/ThyLeader/fishy/database"

	"github.com/iopred/discordgo"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// Fish route (POST /v1/fish/:guildID). Handles the fish command.
func Fish(ctx *fasthttp.RequestCtx) {
	var msg *discordgo.Message
	if err := unmarshalRequest(ctx, msg); err != nil {
		respondMessage(ctx, true, fmt.Sprintf("Error reading and unmarshaling request\n%v", err.Error()))
		return
	}

	// Track user and command usage
	go database.CmdStats("fishy", msg.ID)
	go database.TrackUser(msg.Author)

	// Check fishing eligibility (not blacklisted, not gathering, not ratelimited)
	if database.CheckBlacklist(msg.Author.ID) {
		respondMessage(ctx, false,
			fmt.Sprintf(
				":x: | User %v#%v has been blacklisted from fishing.",
				msg.Author.Username,
				msg.Author.Discriminator,
			),
		)
		return
	}
	if gathering, timeLeft := database.CheckGatherBait(msg.Author.ID); gathering {
		respondMessage(ctx, false,
			fmt.Sprintf(
				":x: | You are currently gathering bait. Please wait %v for you to finish.",
				timeLeft.String(),
			),
		)
		return
	}
	if rl, timeLeft := database.CheckCommandRateLimit("fishy", msg.Author.ID); rl {
		respondMessage(ctx, false,
			fmt.Sprintf(
				":x: | Please wait %v before fishing again!",
				timeLeft.String(),
			),
		)
		return
	}

	// Check for missing inventory items
	noinv := database.CheckMissingInventory(msg.Author.ID)
	if len(noinv) > 0 {
		sort.Strings(noinv)

		if i := sort.SearchStrings(noinv, "rod"); i < len(noinv) && noinv[i] == "rod" {
			database.IncrementInvEE(msg.Author.ID)
			a := database.GetInvEE(msg.Author.ID)
			num := math.Floor(float64(a / 10))
			respondMessage(ctx, false, secrets.InvEE[int(num)])
			if num == float64(len(secrets.InvEE))-1 {
				database.EditItemTier(msg.Author.ID, "rod", "1")
				database.EditItemTier(msg.Author.ID, "hook", "1")
			}
			return
		}
		if i := sort.SearchStrings(noinv, "hook"); i < len(noinv) && noinv[i] == "hook" {
			respondMessage(ctx, false,
				fmt.Sprint(
					"You cast your line but it just sits on the surface\n"+
						"*Something inside of you thinks that fish won't bite without a hook...*",
				),
			)
			return
		}
		respondMessage(ctx, false,
			fmt.Sprintf(
				"You do not own the correct equipment for fishing\n"+
					"Please buy the following items: %v",
				strings.Join(noinv, ", "),
			),
		)
		return
	}

	// Check bait
	amt, err := database.GetCurrentBaitAmt(msg.Author.ID)
	if err != nil {
		respondMessage(ctx, true,
			fmt.Sprintf("There was an error"),
		)
		log.WithField("err", err).Warn("error converting current bait tier")
		return
	}
	if amt < 1 {
		respondMessage(ctx, false,
			"You do not own any bait of your currently equipped tier. Please buy more bait or switch tiers.",
		)
		return
	}

	// Get location information for user (location name, density, bite rate, fish rate)
	loc, err := database.GetLocation(msg.Author.ID)
	if err != nil {
		respondMessage(ctx, true,
			fmt.Sprintf("There was an error"),
		)
		log.WithField("err", err).Warn("failed to get user location")
		return
	}
	density, _ := database.GetLocDensity(msg.Author.ID)
	bite := database.GetBiteRate(msg.Author.ID, density, loc)
	catch, err := database.GetCatchRate(msg.Author.ID)
	if err != nil {
		respondMessage(ctx, true, err.Error())
		return
	}
	fish, err := database.GetFishRate(msg.Author.ID)
	if err != nil {
		respondMessage(ctx, true, err.Error())
		return
	}

	// Increment guild and global cast stats
	go database.IncrementGuildCastStats(msg.Author.ID, ctx.UserValue("guildID").(string))
	go database.IncrementGlobalCastStats(msg.Author.ID)

	// Cast and determine catch
	fc, e := fishCatch(bite, catch, fish)
	if fc {
		if e == "garbage" {
			// Caught garbage
			go database.AddFishToInv(msg.Author.ID, "garbage", 5)
			go database.IncrementGuildGarbageStats(msg.Author.ID, ctx.UserValue("guildID").(string))
			go database.IncrementGlobalGarbageStats(msg.Author.ID)

			respond(ctx, makeEmbedTrash(msg.Author.Username, loc, randomTrash(), density))
			log.WithFields(log.Fields{
				"user":     msg.Author.ID,
				"guild":    ctx.UserValue("guildID").(string),
				"location": loc,
				"rates": map[string]interface{}{
					"bite":  bite,
					"catch": catch,
					"fish":  fish,
				},
				"density": density,
			}).Debug("garbage-catch")
		} else if e == "fish" {
			// Caught a fish
			level := ExpToTier(database.GetGlobalScore(msg.Author.ID))
			f := getFish(level, loc)
			go database.AddGuildAvgFishStats(msg.Author.ID, ctx.UserValue("guildID").(string), f.Size)
			go database.AddGlobalAvgFishStats(msg.Author.ID, f.Size)

			err := database.AddFishToInv(msg.Author.ID, "fish", f.Price)
			if err != nil {
				respondMessage(ctx, false,
					"Your fish inventory is full and you cannot carry any more. You are forced to throw the fish back.",
				)
			} else {
				go database.IncrementGlobalScore(msg.Author.ID, 1)
				go database.DecrementBait(msg.Author.ID)
				newDen, _ := database.IncreaseRandomLocDensity(loc, msg.Author.ID)
				respond(ctx, makeEmbedFish(f, msg.Author.Username, newDen))
				log.WithFields(log.Fields{
					"user":     msg.Author.ID,
					"guild":    ctx.UserValue("guildID").(string),
					"fish-len": f.Size,
					"price":    f.Price,
					"tier":     f.Tier,
					"rates": map[string]interface{}{
						"bite":  bite,
						"catch": catch,
						"fish":  fish,
					},
					"density": density,
				}).Debug("fish-catch")
			}
		}
		return
	}

	// Failed to catch a fish
	respond(ctx, makeEmbedFail(msg.Author.Username, loc, failed(e, msg.Author.ID), density))
	log.WithFields(log.Fields{
		"user":  msg.Author.ID,
		"guild": ctx.UserValue("guildID").(string),
		"rates": map[string]interface{}{
			"bite":  bite,
			"catch": catch,
			"fish":  fish,
		},
		"density": density,
	}).Debug("fail-catch")
}

func fishCatch(bite, catch, fish int64) (bool, string) {
	var r1, r2, r3 int64
	if r, err := rand.Int(rand.Reader, big.NewInt(99)); err == nil {
		r1 = r.Int64()
	}
	if r, err := rand.Int(rand.Reader, big.NewInt(99)); err == nil {
		r2 = r.Int64()
	}
	if r, err := rand.Int(rand.Reader, big.NewInt(99)); err == nil {
		r3 = r.Int64()
	}

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

func randomTrash() string {
	r, err := rand.Int(rand.Reader, big.NewInt(int64(len(trash.Regular.Text)-1)))
	if err != nil {
		log.WithField("err", err).Error("failed to generate random number to pick trash string")
		return "garbage"
	}
	return trash.Regular.Text[int(r.Int64())]
}

func makeEmbedFail(user, location, fail string, locDen database.UserLocDensity) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		//Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: "https://cdn.discordapp.com/attachments/288505799905378304/332261752777736193/Can.png"},
		Color:       0xFF0000,
		Title:       fmt.Sprintf("%s, you were unable to catch anything", user),
		Description: fail,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%+v", locDen),
		},
	}
}

func makeEmbedTrash(user, location, trash string, locDen database.UserLocDensity) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: "https://cdn.discordapp.com/attachments/288505799905378304/332261752777736193/Can.png"},
		Color:       0xffffff,
		Title:       fmt.Sprintf("%s, you fished up some trash in the %s", user, location),
		Description: fmt.Sprintf("It's %s", trash),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%+v", locDen),
		},
	}
}

func makeEmbedFish(fish database.InvFish, user string, locDen database.UserLocDensity) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: fish.URL},
		Color:       tierToEmbedColor(fish.Tier),
		Title:       fmt.Sprintf("%s, you caught a %s in the %s", user, fish.Name, fish.Location),
		Description: fish.Pun,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Length", Value: fmt.Sprintf("%.2fcm", fish.Size), Inline: false},
			&discordgo.MessageEmbedField{Name: "Price", Value: fmt.Sprintf("%.0f¥", fish.Price), Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%+v", locDen),
		},
	}
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

func getFish(tier int, location string) database.InvFish {
	_tier := selectTier(tier)
	base := fish.Location.Ocean
	switch location {
	case "lake":
		base = fish.Location.Lake
	case "river":
		base = fish.Location.River
	}
	fish := base[_tier-1].Fish
	var rand1, rand2 int64
	// fish number
	if r, err := rand.Int(rand.Reader, big.NewInt(int64(len(fish)-1))); err == nil {
		rand1 = r.Int64()
	}
	_fish := fish[int(rand1)]
	// fish len
	if r, err := rand.Int(rand.Reader, big.NewInt(int64(_fish.Size[1]-_fish.Size[0]))); err == nil {
		rand2 = r.Int64() + int64(_fish.Size[0])
	}
	r := float64(rand2)
	r += pRand.Float64()
	sellPrice := getFishPrice(_tier, float64(_fish.Size[0]), float64(_fish.Size[1]), r)
	log.WithFields(log.Fields{
		"tier":     _tier,
		"location": location,
		"fish": map[string]interface{}{
			"name":  _fish.Name,
			"size":  r,
			"price": sellPrice,
		},
	}).Debug("rand-fish")
	return database.InvFish{location, _fish.Name, sellPrice, r, _tier, _fish.Pun, _fish.Image}
}

func getFishPrice(tier int, min, max, l float64) float64 {
	var ratio, price float64
	switch tier {
	case 1:
		ratio = (l - min) / (max - min)
		price = ((fish.Prices[0][1] - fish.Prices[0][0]) * ratio) + fish.Prices[0][0]
	case 2:
		ratio = (l - min) / (max - min)
		price = ((fish.Prices[1][1] - fish.Prices[1][0]) * ratio) + fish.Prices[1][0]
	case 3:
		ratio = (l - min) / (max - min)
		price = ((fish.Prices[2][1] - fish.Prices[2][0]) * ratio) + fish.Prices[2][0]
	case 4:
		ratio = (l - min) / (max - min)
		price = ((fish.Prices[3][1] - fish.Prices[3][0]) * ratio) + fish.Prices[3][0]
	case 5:
		ratio = (l - min) / (max - min)
		price = ((fish.Prices[4][1] - fish.Prices[4][0]) * ratio) + fish.Prices[4][0]
	default:
		log.WithFields(log.Fields{
			"err":  errors.New("unknown tier in price calculation"),
			"tier": tier,
			"max":  max,
			"min":  min,
			"l":    l,
		}).Warn("failed to get fish price")
		return price
	}

	return math.Floor(price)
}

func failed(e, uID string) string {
	if e == "catch" {
		go database.DecrementBait(uID)
		return "a fish bit but you were unable to wrangle it in"
	}
	if e == "bite" {
		return "you couldn't get a fish to bite"
	}
	return ""
}

func selectTier(userTier int) int {
	switch userTier {
	case 1:
		return 1

	case 2:
		sel, err := rand.Int(rand.Reader, big.NewInt(int64(t2Total)))
		if err != nil {
			log.WithField("err", err).Error("failed to generate random number")
			return 0
		}
		switch {
		case int(sel.Int64()) <= t1Total:
			return 1
		default:
			return 2
		}

	case 3:
		sel, err := rand.Int(rand.Reader, big.NewInt(int64(t2Total)))
		if err != nil {
			log.WithField("err", err).Error("failed to generate random number")
			return 0
		}
		switch {
		case int(sel.Int64()) <= t1Total:
			return 1
		case int(sel.Int64()) <= t2Total:
			return 2
		default:
			return 3
		}

	case 4:
		sel, err := rand.Int(rand.Reader, big.NewInt(int64(t2Total)))
		if err != nil {
			log.WithField("err", err).Error("failed to generate random number")
			return 0
		}
		switch {
		case int(sel.Int64()) <= t1Total:
			return 1
		case int(sel.Int64()) <= t2Total:
			return 2
		case int(sel.Int64()) <= t3Total:
			return 3
		default:
			return 4
		}

	default:
		sel, err := rand.Int(rand.Reader, big.NewInt(int64(t2Total)))
		if err != nil {
			log.WithField("err", err).Error("failed to generate random number")
			return 0
		}
		switch {
		case int(sel.Int64()) <= t1Total:
			return 1
		case int(sel.Int64()) <= t2Total:
			return 2
		case int(sel.Int64()) <= t3Total:
			return 3
		case int(sel.Int64()) <= t4Total:
			return 4
		default:
			return 5
		}
	}
}

func ExpToTier(e float64) int {
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
