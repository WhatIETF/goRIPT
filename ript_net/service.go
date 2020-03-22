package ript_net

import (
	"github.com/WhatIETF/goRIPT/api"
	"github.com/google/uuid"
	"log"
)

// RIPT Service Business Logic

// todo: move this to common place
const (
	baseUrl                     = "/.well-known/ript/v1"
	baseTtrunkGroupsUrl         = baseUrl + "/providertgs"
	defaultTrunkGroupId         = "trunkabc"
	trunkGroupDirectionOutbound = "outbound"
)

type TrunkGroupDirection string

type Handler struct {
	id     string
	adInfo api.AdvertisementInfo
	uri    string
}

// capture service representation
// direction, allowed identities, allowed numbers, media capabilities of the service
type Call struct {
}

/// local cache (replace this with db or file/json store)
type RIPTService struct {
	trunkGroups map[string][]api.TrunkGroup
	handlers    map[string]Handler
}

func NewRIPTService() *RIPTService {
	// TODO:  harcoded provider trunkGroupInfo, provision it via API
	tgs := map[string][]api.TrunkGroup{
		trunkGroupDirectionOutbound: {
			{Uri: baseTtrunkGroupsUrl + "/" + defaultTrunkGroupId},
		},
	}

	return &RIPTService{
		trunkGroups: tgs,
		handlers:    map[string]Handler{},
	}
}

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

	return api.TrunkGroupsInfoMessage{
		TrunkGroups: s.trunkGroups,
	}
}

func (s *RIPTService) registerHandler(message api.RegisterHandlerMessage) (api.RegisterHandlerMessage, error) {
	hId, err := uuid.NewUUID()
	if err != nil {
		return api.RegisterHandlerMessage{}, nil
	}

	uri := baseTtrunkGroupsUrl + "/" + defaultTrunkGroupId + "/" + hId.String()
	ad := api.Advertisement(message.HandlerRequest.Advertisement)
	parsed, err := ad.Parse()
	if err != nil {
		return api.RegisterHandlerMessage{}, nil
	}

	h := Handler{
		id:     message.HandlerRequest.HandlerId,
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
