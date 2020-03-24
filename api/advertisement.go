package api

import (
	"errors"
	"fmt"
	"strconv"
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
	Id        uint8
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
		if len(parts) < 3 {
			// need id, dir and atleast one codec
			return adInfo, fmt.Errorf("line [%d] misses minimally required slots", idx)
		}

		id, err := strconv.Atoi(parts[0])
		if err != nil {
			return adInfo, err
		}

		cap.Id = uint8(id)

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

//////
// Directive
/////

type Directive string

func (d Directive) Parse() (DirectiveInfo, error) {

	dInfo := DirectiveInfo{}

	sDirective := string(d)
	parts := strings.Split(sDirective, ":")

	log.Printf("\tparts [%s]", parts)

	if len(parts) < 2 {
		// need  src to sink, codecinfo
		return DirectiveInfo{}, fmt.Errorf("misses minimally required slots")
	}

	srcDst := strings.Split(parts[0], "to")
	src, err := strconv.Atoi(strings.TrimRight(srcDst[0], " "))
	if err != nil {
		return DirectiveInfo{}, err
	}

	dst, err := strconv.Atoi(strings.TrimLeft(srcDst[1], " "))
	if err != nil {
		return DirectiveInfo{}, err
	}

	dInfo.SourceId = uint8(src)
	dInfo.SinkId = uint8(dst)
	codec := strings.TrimRight(parts[1], ";")
	dInfo.Codec = CodecInfo{codec}

	return dInfo, nil
}

type DirectiveInfo struct {
	SourceId uint8
	SinkId   uint8
	Codec    CodecInfo
}

func (di DirectiveInfo) GenClientDirectives() Directive {
	d := strconv.Itoa(int(di.SinkId)) + " to " + strconv.Itoa(int(di.SourceId)) + ":" + di.Codec.Codec + ";"
	return Directive(d)
}

func (di DirectiveInfo) GenServerDirectives() Directive {
	d := strconv.Itoa(int(di.SourceId)) + " to " + strconv.Itoa(int(di.SinkId)) + ":" + di.Codec.Codec + ";"
	return Directive(d)
}
