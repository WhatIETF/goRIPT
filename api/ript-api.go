package api

// API definitions for ript
// TODO: Use RAML/Swagger for auto generating the code

const (
	TrunkGroupDiscoveryPacket PacketType = 1
	RegisterHandlerPacket     PacketType = 2
	CallsPacket               PacketType = 3
	ContentPacket             PacketType = 4
	StreamMediaPacket         PacketType = 5
	StreamMediaAckPacket      PacketType = 6
	StreamMediaRequestPacket  PacketType = 7
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
	Type               PacketType
	Filter             ContentFilter
	Content            ContentMessage
	RegisterHandler    RegisterHandlerMessage
	TrunkGroupsInfo    TrunkGroupsInfoMessage
	Calls              CallsMessage
	StreamMedia        StreamContentMedia
	StreamMediaAck     Acknowledgement
	StreamMediaRequest StreamContentRequest
}

type PacketEvent struct {
	Sender FaceName
	TgId   string
	CallId string
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

///// Media
const (
	// stream content types (media/control)
	StreamContentTypeMedia   = 0
	StreamContentTypeControl = 1

	// media codec types
	PayloadTypeOpus = 1

	// control message types
	StreamContentControlTypeAck = 0
)

type StreamContentType uint8
type StreamContentControlType int16

type StreamContentRequest struct {
}

type StreamContentMedia struct {
	Type        StreamContentType
	SeqNo       uint64
	Timestamp   uint64
	PayloadType uint32
	SourceId    uint8
	SinkId      uint8
	Media       []byte `tls:"head=varint"`
}

type Acknowledgement struct {
	Type        StreamContentType //media/control
	ControlType StreamContentControlType
	Direction   string
	SourceId    uint8
	SinkId      uint8
}
