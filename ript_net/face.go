package ript_net

//  Abstract interface for the underlying transport

type Face interface {
	Name() FaceName
	Send(pkt Packet) error
	Read()
	SetReceiveChan(recv chan PacketEvent)
	Close(err error)
	OnClose() chan error
	CanStream() bool
}

type FaceFactory interface {
	Feed() chan Face
}
