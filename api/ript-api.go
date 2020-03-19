package api

import "crypto/sha1"

// API definitions for ript
// TODO: Use RAML/Swagger for auto generating the code


type HandlerRequest struct {
	HandlerId string `json:"handler-id"`
	Advertisement string `json:"advertisement"`
}

type HandlerResponse struct {
	Uri string `json:"uri"`
}

const (
	ContentPacket        PacketType = 1
	ContentRequestPacket PacketType = 2
	RegisterHandlerPacket PacketType = 3
)

const (
	ContentFilterMediaForward ContentFilter = 1
	ContentFilterMediaReverse ContentFilter = 2
)

type FaceName string
type PacketType byte
type ContentFilter byte
type DeliveryAddress string
type ContentHash []byte

type ContentMessage struct {
	To DeliveryAddress
	Id int32
	Content []byte
}

func (msg ContentMessage) Hash() ContentHash {
	h := sha1.New()
	h.Write([]byte(msg.Content))
	return ContentHash(h.Sum(nil))
}

type ContentRequestMessage struct {
	To DeliveryAddress
	Id int32
}

type RegisterHandlerMessage struct {
	HandlerRequest HandlerRequest
	HandlerResponse HandlerResponse
}

type Packet struct {
	Type PacketType
	Filter ContentFilter
	Content ContentMessage
	ContentRequest ContentRequestMessage
	RegisterHandler RegisterHandlerMessage
}

type PacketEvent struct {
	Sender FaceName
	Packet Packet
}

////
//// ript semantics
////

type Advertisement string

/// Handler
type HandlerInfo struct {
	Id string
	Advertisement Advertisement
	Uri string
}

func (h HandlerInfo) matchCaps(other HandlerInfo) bool {
	// exact match
	// TODO: fix this for full cap-adv framework
	if other.Advertisement == h.Advertisement {
		return true
	}
	return false
}