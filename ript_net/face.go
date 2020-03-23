package ript_net

import "github.com/WhatIETF/goRIPT/api"

// Abstract interface for the underlying transport
type Face interface {
	Name() api.FaceName
	Send(pkt api.Packet) error
	Read()
	SetReceiveChan(recv chan api.PacketEvent)
	Close(err error)
	OnClose() chan error
	CanStream() bool
}

type FaceFactory interface {
	Feed() chan Face
}
