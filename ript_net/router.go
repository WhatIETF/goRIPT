package ript_net

import (
	"log"
	"sync"

	"github.com/WhatIETF/goRIPT/api"
)

/////

type Router struct {
	name     string
	faceLock sync.Mutex
	faces    map[api.FaceName]Face
	recvChan chan api.PacketEvent
	service  *RIPTService
}

func NewRouter(name string, service *RIPTService) *Router {
	r := &Router{
		name:     name,
		faces:    map[api.FaceName]Face{},
		recvChan: make(chan api.PacketEvent, 200),
		service:  service,
	}
	go r.route()
	return r
}

//TODO: Handle Error reporting
func (r *Router) route() {
	for evt := range r.recvChan {
		log.Printf("[%s] received from [%s], packet %v", r.name, evt.Sender, evt.Packet.Type)

		switch evt.Packet.Type {
		case api.TrunkGroupDiscoveryPacket:
			log.Printf("ript_net: handle /trunkGroupDiscovery.")
			response := r.service.listTrunkGroups()
			packet := api.Packet{
				Type:            api.TrunkGroupDiscoveryPacket,
				TrunkGroupsInfo: response,
			}

			err := r.faces[evt.Sender].Send(packet)
			if err != nil {
				r.RemoveFace(r.faces[evt.Sender], err)
			}
			continue

		case api.RegisterHandlerPacket:
			// handler registration
			log.Printf("ript_net: handle /handlerRegistration.")
			response, _ := r.service.registerHandler(evt.Packet.RegisterHandler)
			packet := api.Packet{
				Type:            api.RegisterHandlerPacket,
				RegisterHandler: response,
			}

			err := r.faces[evt.Sender].Send(packet)
			if err != nil {
				r.RemoveFace(r.faces[evt.Sender], err)
			}
			continue

		case api.CallsPacket:
			log.Printf("ript_net: handle /calls.")
			response, _ := r.service.processCalls(evt.TgId, evt.Packet.Calls)
			packet := api.Packet{
				Type:  api.CallsPacket,
				Calls: response,
			}

			err := r.faces[evt.Sender].Send(packet)
			if err != nil {
				r.RemoveFace(r.faces[evt.Sender], err)
			}
			continue

		case api.StreamMediaPacket:
			m := evt.Packet.StreamMedia
			log.Printf("ript_net: handle /mediaForward. SourceId [%v], SinkId [%v], SeqNo [%v]",
				m.SourceId, m.SinkId, m.SeqNo)

			for name, face := range r.faces {
				if name == evt.Sender {
					continue
				}
				log.Printf("[%s] forwarding Content [%d] on [%s]", r.name, m.SeqNo, name)
				err := face.Send(evt.Packet)
				if err != nil {
					r.RemoveFace(face, err)
				}
			}
			continue

		default:
			log.Fatalf("unknown packet type [%s]", evt.Packet.Type)
		}
	}
}

func (r *Router) RemoveFace(face Face, err error) {
	r.faceLock.Lock()
	log.Printf("[%s] Removing face [%s] [%v]", r.name, face.Name(), err)
	delete(r.faces, face.Name())
	r.faceLock.Unlock()
}

func (r *Router) AddFace(face Face) {
	r.faceLock.Lock()
	log.Printf("[%s] Adding face [%s]\n", r.name, face.Name())
	face.SetReceiveChan(r.recvChan)
	r.faces[face.Name()] = face
	r.faceLock.Unlock()

	go r.awaitFaceClose(face)
}

func (r *Router) awaitFaceClose(face Face) {
	err := <-face.OnClose()
	r.RemoveFace(face, err)
}

func (r *Router) AddFaceFactory(factory FaceFactory) {
	go r.readFaceFeed(factory.Feed())
}

func (r *Router) readFaceFeed(faceChan chan Face) {
	for face := range faceChan {
		r.AddFace(face)
	}
}
