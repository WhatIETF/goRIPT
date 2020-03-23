package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-acme/lego/log"
)

const (
	DirectionIn  = "in"
	DirectionOut = "out"
)

// todo: support codec params
type CodecInfo struct {
	Codec string
}

func (ci CodecInfo) Match(other CodecInfo) bool {
	// Code name match alone
	// TODO: revisit this once we have more info on Cap/Adv model
	if ci.Codec == other.Codec {
		return true
	}
	return false
}

type Capability struct {
	Id        string
	Direction string
	Codecs    []CodecInfo
}

type AdvertisementInfo struct {
	Caps []Capability
}

// Crude parse function. Needs to be revisited once ABNF is defined
func (ad Advertisement) Parse() (AdvertisementInfo, error) {

	adLines := strings.Split(string(ad), "\n")
	if len(adLines) == 0 {
		return AdvertisementInfo{}, errors.New("error parsing advertisement")
	}

	log.Println("lines [%s]", adLines)
	adInfo := AdvertisementInfo{}
	for idx, line := range adLines {
		var cap Capability
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, " ")
		log.Printf("\tparts [%s]", parts)

		if len(parts) < 3 {
			// need id, dir and atleast one codec
			return adInfo, fmt.Errorf("line [%d] misses minimally required slots", idx)
		}

		id := parts[0]
		cap.Id = id

		dir := strings.TrimRight(parts[1], ":")

		if dir != DirectionIn && dir != DirectionOut {
			return adInfo, fmt.Errorf("line [%d] malformed direction", idx)
		}
		cap.Direction = dir

		// parse codes
		var codecs []CodecInfo
		for i := 2; i < len(parts); i++ {
			codec := strings.TrimRight(parts[i], ";")
			codecs = append(codecs, CodecInfo{codec})
		}
		cap.Codecs = codecs
		adInfo.Caps = append(adInfo.Caps, cap)
	}
	return adInfo, nil
}
