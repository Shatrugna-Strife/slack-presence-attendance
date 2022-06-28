package data

import (
	"log"
	"time"
)

type Presence string

const (
	Active Presence = "active"
	Away   Presence = "away"
)

type UserTimeData struct {
	UserId        string
	PresenceState Presence
	Name          string
	LastChecked   int64
	ActiveEpoch   int64
	AwayEpoch     int64
	TotalDuration int64
}

type TimeStampCustom struct {
	Hour   int // Hours - 24 format
	Minute int
}

func test() {
	t1 := time.Now()
	log.Println(t1.Unix())
}
