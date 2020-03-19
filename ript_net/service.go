package ript_net

import (
	"github.com/WhatIETF/goRIPT/api"
	"github.com/google/uuid"
	"log"
)
// RIPT Service Business Logic

// todo: move this to common place
const (
	defaultTrunkGroupId = "trunk123"
)

/// local cache (replace this with db or file/json store)
type RIPTService struct {
	handlers map[string]api.HandlerInfo
}

func NewRIPTService() *RIPTService {
	return &RIPTService{
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

func (s *RIPTService) registerHandler(message api.RegisterHandlerMessage) (api.RegisterHandlerMessage, error) {
	hId, err := uuid.NewUUID()
	if err != nil {
		return api.RegisterHandlerMessage{}, nil
	}
	uri := "localhost:6121/.well-known/ript/v1/providertgs/trunk123/handlers/"+ hId.String()

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