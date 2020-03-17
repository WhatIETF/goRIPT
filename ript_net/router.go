package ript_net

import (
	"crypto/sha1"
	"log"
	"sync"
)

const (
	ContentPacket        PacketType = 1
	ContentRequestPacket PacketType = 2
)

type FaceName string
type PacketType byte
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

type Packet struct {
	Type PacketType
	Content ContentMessage
	ContentRequest ContentRequestMessage
}

type PacketEvent struct {
	Sender FaceName
	Packet Packet
}


//// Packet Cache
// todo(suhas): implement cache truncation
type Cache struct {
	sync.Mutex
	currentId int32
	cache map[DeliveryAddress]map[int32]ContentMessage
}

func newCache() *Cache {
	return &Cache{
		cache: map[DeliveryAddress]map[int32]ContentMessage{},
	}
}

func (c *Cache) Add(msg ContentMessage) {
	c.Lock()
	defer c.Unlock()
	_, ok := c.cache[msg.To]
	if !ok {
		c.cache[msg.To] = map[int32]ContentMessage{}
	}
	c.currentId = msg.Id
	c.cache[msg.To][c.currentId] = msg
}

func (c Cache) Get(addr DeliveryAddress, id int32) (ContentMessage, bool) {
	messages := c.cache[addr]
	if len(messages) == 0 {
		return ContentMessage{}, false
	}

	if id == -1 {
		id = c.currentId
	}
	msg := c.cache[addr][id]
	if len(msg.Content) == 0 {
		return ContentMessage{}, false
	}

	return msg, true
}

func (c Cache) Flush() {
	c.cache = map[DeliveryAddress]map[int32]ContentMessage{}
}

/////

type Router struct {
	name     string
	faceLock sync.Mutex
	faces    map[FaceName]Face
	recvChan chan PacketEvent
	cache *Cache
}

func NewRouter(name string) *Router {
	r := &Router{
		name:     name,
		faces:    map[FaceName]Face{},
		recvChan: make(chan PacketEvent, 200),
		cache: newCache(),
	}
	go r.route()
	return r
}

func (r *Router) route() {
	for evt := range r.recvChan {
		log.Printf("[%s] received from [%s], packet %v", r.name, evt.Sender, evt.Packet.Type)

		switch evt.Packet.Type {
		case ContentPacket:
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

		case ContentRequestPacket:
			addr := evt.Packet.ContentRequest.To
			id := evt.Packet.ContentRequest.Id
			log.Printf("Looking cachne for content id [%d]", id)
			msg, ok := r.cache.Get(addr, id)
			if !ok {
				log.Printf("no cached media found for [%v]", addr)
				continue
			}

			// send the packet to the requestor
			packet := Packet{
				Type: ContentPacket,
				Content: msg,
			}

			log.Printf("[%s] forwarding Content Id [%d], len %d on [%s]\n", r.name, msg.Id, len(msg.Content), evt.Sender)
			log.Printf("raw content [%v]", msg)

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
