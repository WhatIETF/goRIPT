package ript_net

import (
	"github.com/WhatIETF/goRIPT/api"
	"github.com/google/uuid"
	"log"
)
// RIPT Service Business Logic

// todo: move this to common place
const (
	baseUrl = "/.well-known/ript/v1"
	baseTtrunkGroupsUrl = baseUrl + "/providertgs"
	defaultTrunkGroupId = "trunkabc"
	trunkGroupDirectionOutbound = "outbound"
)

type TrunkGroupDirection string

// capture service representation
// direction, allowed identities, allowed numbers, media capabilities of the service
type Call struct {

}


/// local cache (replace this with db or file/json store)
type RIPTService struct {
	trunkGroups map[string][]api.TrunkGroup
	handlers map[string]api.HandlerInfo
}

func NewRIPTService() *RIPTService {
	// TODO:  harcoded provider trunkGroupInfo, provision it via API
	tgs := map[string][]api.TrunkGroup{
		trunkGroupDirectionOutbound: []api.TrunkGroup{
			{Uri: baseTtrunkGroupsUrl + "/" + defaultTrunkGroupId},
		},
	}

	return &RIPTService{
		trunkGroups:tgs,
		handlers: map[string]api.HandlerInfo{},
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

	uri := baseUrl + "providertgs/trunk123/handlers/"+ hId.String()

	h := api.HandlerInfo{
		Id: message.HandlerRequest.HandlerId,
		Advertisement: api.Advertisement(message.HandlerRequest.Advertisement),
		Uri: uri,
	}

	s.handlers[message.HandlerRequest.HandlerId] = h

	// send the response message
	return api.RegisterHandlerMessage{
		HandlerResponse: api.HandlerResponse{
			Uri: uri,
		},
	}, nil
}