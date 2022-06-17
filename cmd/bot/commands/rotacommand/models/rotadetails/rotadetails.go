package rotadetails

import (
	"alfred-bot/utils/formatter"
	"time"
)

type RotaDetails struct {
	Pk               string // ChannelID
	Sk               string // RotaName
	Members          []string
	CurrOnCallMember string
	Duration         int
	StartOfShift     string
	EndOfShift       string
}

func (rd *RotaDetails) RotaName() string {
	return rd.Sk
}

func GenerateEndOfShift(startOfShift time.Time, duration int) string {
	// TODO: Hard-coded quick shift duration for testing purposes
	return formatter.FormatTime(startOfShift.Add(time.Minute))
	// return formatTime(startOfShift.Add(time.Hour * 168 * time.Duration(rd.Duration)))
}
