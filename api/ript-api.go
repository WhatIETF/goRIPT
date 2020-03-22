package api

// API definitions for ript
// TODO: Use RAML/Swagger for auto generating the code


const (
	TrunkGroupDiscoveryPacket PacketType = 1
	RegisterHandlerPacket PacketType = 2
	ContentPacket        PacketType = 3
)

// types of content carried by the Content Paclet
const (
	ContentFilterMediaForward ContentFilter = 1
	ContentFilterMediaReverse ContentFilter = 2
)

type FaceName string
type PacketType byte
type ContentFilter byte
type DeliveryAddress string

type ContentMessage struct {
	To DeliveryAddress
	Id int32
	Content []byte
}

type Packet struct {
	Type PacketType
	Filter ContentFilter
	Content ContentMessage
	RegisterHandler RegisterHandlerMessage
	TrunkGroupsInfo TrunkGroupsInfoMessage
}

type PacketEvent struct {
	Sender FaceName
	Packet Packet
}

////
//// ript semantics
////

type Advertisement string

/// Handler Definition
type HandlerInfo struct {
	Id string
	Advertisement Advertisement
	Uri string
}

func (h HandlerInfo) matchCaps(other HandlerInfo) bool {
	// exact match
	// TODO: fix this for full cap-adv framework
	return true
}

type HandlerRequest struct {
	HandlerId string `json:"handler-id"`
	Advertisement string `json:"advertisement"`
}

type HandlerResponse struct {
	Uri string `json:"uri"`
}

type RegisterHandlerMessage struct {
	HandlerRequest HandlerRequest
	HandlerResponse HandlerResponse
}

//// Trunk

type TrunkGroup struct {
	Uri string
}

type TrunkGroupsInfoMessage struct {
	TrunkGroups map[string][]TrunkGroup
}