package database

import log "github.com/sirupsen/logrus"

// TODO: move
var defaultUserItems = UserItems{
	UserItem{0, []int{}},
	UserItem{0, []int{}},
	UserItem{0, []int{}},
	UserItem{0, []int{}},
	UserItem{0, []int{}},
}

// TODO: move
var allowedItems = map[string]bool{
	"rod":     true,
	"hook":    true,
	"vehicle": true,
	"baitbox": true,
	"bait":    true,
}

// GetBiteRate returns the bite rate for a given user.
func GetBiteRate(userID string, locDen UserLocDensity, loc string) int64 {
	switch loc {
	case "lake":
		return calcBiteRate(int64(locDen.Lake))

	case "river":
		return calcBiteRate(int64(locDen.River))

	case "ocean":
		return calcBiteRate(int64(locDen.Ocean))
	}
	log.WithFields(log.Fields{
		"User":     userID,
		"Location": loc,
	}).Error("User does not have a known location")
	return 0
}

//
func GetCatchRate(userID string) (int64, error) {
	inv, err := GetInventory(userID)
	if err != nil {
		return 0, err
	}
	switch inv.Rod.Current {
	case 200:
		return 50, nil
	case 201:
		return 55, nil
	case 202:
		return 60, nil
	case 203:
		return 70, nil
	case 204:
		return 80, nil
	}
	return 50, nil
}

//
func GetFishRate(userID string) (int64, error) {
	inv, err := GetInventory(userID)
	if err != nil {
		return 0, err
	}
	switch inv.Hook.Current {
	case 300:
		return 50, nil
	case 301:
		return 60, nil
	case 302:
		return 70, nil
	case 303:
		return 80, nil
	case 304:
		return 90, nil
	}
	return 50, nil
}

func calcBiteRate(density int64) (rate int64) {
	if density == 100 {
		rate = 50
		return
	}

	if density < 100 {
		rate = int64((float32(0.4) * float32(density)) + 10.0)
		return
	}

	if density > 100 {
		rate = int64((float32(0.25) * float32(density)) + 25.0)
		return
	}
	return
}
