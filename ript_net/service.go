package ript_net

import (
	"fmt"
	"log"

	"github.com/WhatIETF/goRIPT/api"
	"github.com/google/uuid"
)

// RIPT Service Business Logic

// todo: move this to common place
const (
	baseUrl                     = "/.well-known/ript/v1"
	baseTrunkGroupsUrl          = baseUrl + "/providertgs"
	defaultTrunkGroupId         = "trunkAbc"
	trunkGroupDirectionOutbound = "outbound"
	defaultTrunkMediaCap        = "1 out: opus;\n" + "2 out: opus;\n"
)

type TrunkGroupDirection string

type TrunkGroup struct {
	id        string
	uri       string
	direction string
	mediaCap  api.Advertisement
	call      Call
}

// Handler Information
type Handler struct {
	id     string
	adRaw  api.Advertisement
	adInfo api.AdvertisementInfo
	uri    string
}

// capture service representation
// direction, allowed identities, allowed numbers, media capabilities of the service
type Call struct {
	id  string
	uri string
}

/// local cache (replace this with db or file/json store)
type RIPTService struct {
	trunkGroups map[string]*TrunkGroup
	handlers    map[string]Handler
}

func NewRIPTService() *RIPTService {
	// TODO:  harcoded provider trunkGroupInfo, provision it via API
	tg := &TrunkGroup{
		id:        defaultTrunkGroupId,
		uri:       baseTrunkGroupsUrl + "/" + defaultTrunkGroupId,
		direction: trunkGroupDirectionOutbound,
		mediaCap:  defaultTrunkMediaCap,
	}

	log.Printf("riptService: Created Default TrunkGroup [%v]", tg)
	tgs := map[string]*TrunkGroup{}
	tgs[tg.id] = tg
	return &RIPTService{
		trunkGroups: tgs,
		handlers:    map[string]Handler{},
	}
}

// TODO: handle error processing
func (s *RIPTService) handle(pkt api.Packet) {
	switch pkt.Type {
	case api.RegisterHandlerPacket:
		s.registerHandler(pkt.RegisterHandler)
	default:
		log.Fatalf("riptService: Unknown pakcet type [%v]", pkt.Type)
	}
}

func (s *RIPTService) listTrunkGroups() api.TrunkGroupsInfoMessage {
	if len(s.trunkGroups) == 0 {
		return api.TrunkGroupsInfoMessage{}
	}

	var tgInfo []api.TrunkGroupInfo
	for _, tg := range s.trunkGroups {
		tgInfo = append(tgInfo, api.TrunkGroupInfo{Uri: tg.uri})
	}

	return api.TrunkGroupsInfoMessage{
		TrunkGroups: tgInfo,
	}
}

func (s *RIPTService) registerHandler(message api.RegisterHandlerMessage) (api.RegisterHandlerMessage, error) {
	hId, err := uuid.NewUUID()
	if err != nil {
		return api.RegisterHandlerMessage{}, nil
	}

	uri := baseTrunkGroupsUrl + "/" + defaultTrunkGroupId + "/" + hId.String()
	ad := api.Advertisement(message.HandlerRequest.Advertisement)
	parsed, err := ad.Parse()
	if err != nil {
		return api.RegisterHandlerMessage{}, nil
	}

	h := Handler{
		id:     message.HandlerRequest.HandlerId,
		adRaw:  ad,
		adInfo: parsed,
		uri:    uri,
	}

	log.Printf("service: created handler [%v]", h)

	s.handlers[message.HandlerRequest.HandlerId] = h

	// send the response message
	return api.RegisterHandlerMessage{
		HandlerResponse: api.HandlerResponse{
			Uri: uri,
		},
	}, nil
}

func (s *RIPTService) processCalls(tgId string, message api.CallsMessage) (api.CallsMessage, error) {
	// get tg
	tg, ok := s.trunkGroups[tgId]
	if !ok {
		return api.CallsMessage{}, fmt.Errorf("ript_net: unknown trunkGroupId for /calls")
	}

	handlerUrl := message.Request.HandlerUri
	//retrieve caps for this handler
	var handler Handler
	var found = false
	for _, h := range s.handlers {
		if h.uri == handlerUrl {
			handler = h
			found = true
			break
		}
	}

	if !found {
		return api.CallsMessage{}, fmt.Errorf("ript_net: incorrect handler for /calls")
	}

	// match the caps
	directives, ok := Match(tg.mediaCap, handler.adRaw)
	if !ok {
		return api.CallsMessage{}, fmt.Errorf("ript_net: no matching caps found")
	}

	// since we have a match, generate a new Call object
	callId, err := uuid.NewUUID()
	if err != nil {
		return api.CallsMessage{}, fmt.Errorf("ript_net: callId gen failure")
	}

	uri := baseTrunkGroupsUrl + "/" + tgId + "/calls/" + callId.String()

	call := Call{
		id:  callId.String(),
		uri: uri,
	}

	// save the call on the trunk
	tg.call = call

	response := api.CallResponse{
		CallUri:         call.uri,
		ClientDirective: directives[0].GenServerDirectives(),
		ServerDirective: directives[0].GenServerDirectives(),
	}

	return api.CallsMessage{Response: response}, nil
}

/////
// Helpers
////

// Crude and incorrect match implementation. Needs to be redone
func Match(s, o api.Advertisement) ([]api.DirectiveInfo, bool) {
	source, err := s.Parse()
	if err != nil {
		return nil, false
	}

	target, err := o.Parse()
	if err != nil {
		return nil, false
	}

	var result []api.DirectiveInfo

	for _, scap := range source.Caps {
		for _, tcap := range target.Caps {
			if scap.Direction == tcap.Direction {
				for _, sci := range scap.Codecs {
					for _, tci := range tcap.Codecs {
						if sci.Match(tci) {
							d := api.DirectiveInfo{
								SourceId: scap.Id,
								SinkId:   tcap.Id,
								Codec:    sci,
							}
							result = append(result, d)
							break
						}
					}
				}
			}
		}
	}
	if len(result) == 0 {
		return result, false
	}
	return result, true
}
