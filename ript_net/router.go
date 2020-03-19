package ript_net

import (
	"log"
	"sync"

	"github.com/WhatIETF/goRIPT/api"
)

//// Packet Cache
// todo(suhas): implement cache truncation
type Cache struct {
	sync.Mutex
	currentId int32
	cache map[api.DeliveryAddress]map[int32]api.ContentMessage
}

func newCache() *Cache {
	return &Cache{
		cache: map[api.DeliveryAddress]map[int32]api.ContentMessage{},
	}
}

func (c *Cache) Add(msg api.ContentMessage) {
	c.Lock()
	defer c.Unlock()
	_, ok := c.cache[msg.To]
	if !ok {
		c.cache[msg.To] = map[int32]api.ContentMessage{}
	}
	c.currentId = msg.Id
	c.cache[msg.To][c.currentId] = msg
}

func (c Cache) Get(addr api.DeliveryAddress, id int32) (api.ContentMessage, bool) {
	messages := c.cache[addr]
	if len(messages) == 0 {
		return api.ContentMessage{}, false
	}

	if id == -1 {
		id = c.currentId
	}
	msg := c.cache[addr][id]
	if len(msg.Content) == 0 {
		return api.ContentMessage{}, false
	}

	return msg, true
}

func (c Cache) Flush() {
	c.cache = map[api.DeliveryAddress]map[int32]api.ContentMessage{}
}

/////

type Router struct {
	name     string
	faceLock sync.Mutex
	faces    map[api.FaceName]Face
	recvChan chan api.PacketEvent
	cache *Cache
	service *RIPTService
}

func NewRouter(name string, service *RIPTService) *Router {
	r := &Router{
		name:     name,
		faces:    map[api.FaceName]Face{},
		recvChan: make(chan api.PacketEvent, 200),
		cache: newCache(),
		service: service,
	}
	go r.route()
	return r
}

func (r *Router) route() {
	for evt := range r.recvChan {
		log.Printf("[%s] received from [%s], packet %v", r.name, evt.Sender, evt.Packet.Type)

		switch evt.Packet.Type {
		case api.ContentPacket:
			// add to cache
			// todo: this is blind broadcast, needs to be optimized
			// Forward the packet on all the faces (except sender)
			for name, face := range r.faces {
				if name == evt.Sender {
					continue
				}
				log.Printf("[%s] forwarding Content [%d] on [%s]", r.name, evt.Packet.Content.Id, name)
				err := face.Send(evt.Packet)
				if err != nil {
					r.RemoveFace(face, err)
				}
			}
			continue

		case api.ContentRequestPacket:
			addr := evt.Packet.ContentRequest.To
			id := evt.Packet.ContentRequest.Id
			log.Printf("Looking cachne for content id [%d]", id)
			msg, ok := r.cache.Get(addr, id)
			if !ok {
				log.Printf("no cached media found for [%v]", addr)
				continue
			}

			// send the packet to the requestor
			packet := api.Packet{
				Type: api.ContentPacket,
				Content: msg,
			}

			log.Printf("[%s] forwarding Content Id [%d], len %d on [%s]\n", r.name, msg.Id,
				len(msg.Content), evt.Sender)

			err := r.faces[evt.Sender].Send(packet)
			if err != nil {
				r.RemoveFace(r.faces[evt.Sender], err)
			}
		case api.RegisterHandlerPacket:
			// handler registration
			// todo: handle error
			response, _ := r.service.registerHandler(evt.Packet.RegisterHandler)
			packet :=  api.Packet{
				Type: api.RegisterHandlerPacket,
				RegisterHandler: response,
			}

			err := r.faces[evt.Sender].Send(packet)
			if err != nil {
				r.RemoveFace(r.faces[evt.Sender], err)
			}

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

func (b *Router) AddFaceFactory(factory FaceFactory) {
	go b.readFaceFeed(factory.Feed())
}

func (r *Router) readFaceFeed(faceChan chan Face) {
	for face := range faceChan {
		r.AddFace(face)
	}
}
