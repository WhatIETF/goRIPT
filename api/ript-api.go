package api

// API definitions for ript
// TODO: Use RAML/Swagger for auto generating the code

const (
	TrunkGroupDiscoveryPacket PacketType = 1
	RegisterHandlerPacket     PacketType = 2
	CallsPacket               PacketType = 3
	ContentPacket             PacketType = 4
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
	To      DeliveryAddress
	Id      int32
	Content []byte
}

type Packet struct {
	Type            PacketType
	Filter          ContentFilter
	Content         ContentMessage
	RegisterHandler RegisterHandlerMessage
	TrunkGroupsInfo TrunkGroupsInfoMessage
	Calls           CallsMessage
}

type PacketEvent struct {
	Sender FaceName
	TgId   string
	Packet Packet
}

////
//// ript semantics
////

type Advertisement string

/// Handler Definition
type HandlerInfo struct {
	Id            string
	Advertisement Advertisement
	Uri           string
}

func (h HandlerInfo) matchCaps(other HandlerInfo) bool {
	// exact match
	// TODO: fix this for full cap-adv framework
	return true
}

type HandlerRequest struct {
	HandlerId     string `json:"handler-id"`
	Advertisement string `json:"advertisement"`
}

type HandlerResponse struct {
	Uri string `json:"uri"`
}

type RegisterHandlerMessage struct {
	HandlerRequest  HandlerRequest
	HandlerResponse HandlerResponse
}

//// Trunk

type TrunkGroupInfo struct {
	Uri string
}

type TrunkGroupsInfoMessage struct {
	TrunkGroups []TrunkGroupInfo
}

//// Calls
type CallRequest struct {
	HandlerUri  string `json:"uri"`
	Destination string `json:"destination"`
}

type CallResponse struct {
	CallUri         string `json:"uri"`
	ClientDirective string `json:"clientDirectives"`
	ServerDirective string `json:"serverDirectives"`
}

type CallsMessage struct {
	Request  CallRequest
	Response CallResponse
}
