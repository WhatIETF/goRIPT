package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WhatIETF/goRIPT/api"
	"github.com/WhatIETF/goRIPT/ript_net"
	"github.com/google/uuid"
	"github.com/gordonklaus/portaudio"
)

// info about the provider
type riptProviderInfo struct {
	baseUrl       string
	trunkGroups   []api.TrunkGroupInfo
	activeCallUri string
}

func (p *riptProviderInfo) getTrunkGroupUri() string {
	// pick the first entry as default
	tg := p.trunkGroups[0]
	return tg.Uri
}

// Handler representing an instance of a RIPT Client
type riptClient struct {
	client   ript_net.Face
	stopChan chan bool
	doneChan chan bool
	recvChan chan api.PacketEvent
	// ript protocol semantics
	handlerInfo  api.HandlerInfo
	providerInfo *riptProviderInfo
	callInfo     api.CallResponse
}

// Register this client's device capability with the provider
func (c *riptClient) registerHandler() {
	pkt := api.Packet{
		Type: api.RegisterHandlerPacket,
		RegisterHandler: api.RegisterHandlerMessage{
			HandlerRequest: api.HandlerRequest{
				HandlerId:     c.handlerInfo.Id,
				Advertisement: string(c.handlerInfo.Advertisement),
			},
		},
	}

	err := c.client.Send(pkt)
	if err != nil {
		log.Fatalf("registerHandler:  error [%v]", err)
		panic(err)
	}

	// await response
	select {
	case response := <-c.recvChan:
		c.handlerInfo.Uri = response.Packet.RegisterHandler.HandlerResponse.Uri
	}

	log.Printf("registerHandler: handlerInfo with uri: [%v]", c.handlerInfo)
}

// trigger's call creation on the provider for a given destination
func (c *riptClient) placeCalls() {
	pkt := api.Packet{
		Type: api.CallsPacket,
		Calls: api.CallsMessage{
			Request: api.CallRequest{
				HandlerUri:  c.handlerInfo.Uri,
				Destination: "meeting123@eietf107.ript-dev.com",
			},
		},
	}

	err := c.client.Send(pkt)
	if err != nil {
		log.Fatalf("placeCalls:  error [%v]", err)
		panic(err)
	}

	// await response
	select {
	case response := <-c.recvChan:
		c.callInfo = response.Packet.Calls.Response
	}
	c.providerInfo.activeCallUri = c.callInfo.CallUri
	log.Printf("placeCalls: callInfo: [%v]", c.callInfo)
}

// Bootstrap api to retrieve various available trunkGroups on the RIPT server
func (c *riptClient) retrieveTrunkGroups() {
	pkt := api.Packet{
		Type: api.TrunkGroupDiscoveryPacket,
	}

	err := c.client.Send(pkt)
	if err != nil {
		log.Fatalf("retrieveTrunkGroups:  error [%v]", err)
		panic(err)
	}

	// await response
	select {
	case response := <-c.recvChan:
		c.providerInfo.trunkGroups = response.Packet.TrunkGroupsInfo.TrunkGroups
	}

	log.Printf("trukGroupDiscovery: tgs [%v]", c.providerInfo.trunkGroups)
}

// Record audio from mic and send it to the server over a given transport
func (c *riptClient) recordContent() {
	defer func() {
		c.doneChan <- true
	}()

	mic, err := NewMicrophone()
	chk(err)
	contentChan := make(chan []byte, 1)
	mic.setContentChan(contentChan)
	chk(mic.Start())
	var contentId int32 = 0
	for {
		select {
		case <-c.stopChan:
			log.Println("Recording stopped.")
			_, err := mic.Stop()
			chk(err)
			return
		case content := <-contentChan:
			nanos := time.Now().UnixNano()
			millis := nanos / 1000000
			m := api.StreamContentMedia{
				Type:        api.StreamContentTypeMedia,
				SeqNo:       uint64(contentId),
				Timestamp:   uint64(millis),
				PayloadType: api.PayloadTypeOpus,
				SourceId:    1,
				SinkId:      1,
				Media:       content,
			}

			pkt := api.Packet{
				Type:        api.StreamMediaPacket,
				StreamMedia: m,
			}
			err := c.client.Send(pkt)
			if err != nil {
				log.Fatalf("recordContent: media send error [%v]", err)
				continue
			}
			contentId++
		}
	}
}

// Retrieve media packets from the sever and play it out on the speaker
func (c *riptClient) playOutContent() {
	defer func() {
		c.doneChan <- true
	}()

	c.client.SetReceiveChan(c.recvChan)
	speaker, err := NewSpeaker()
	chk(err)

	for {
		select {
		case <-c.stopChan:
			log.Println("playout stopped")
			return
		case evt := <-c.recvChan:
			log.Printf("got media evt : [%v]", evt)
			speaker.Play(evt.Packet.StreamMedia.Media)
			continue
		default:
			if !c.client.CanStream() {
				// ask server for the packet
				pkt := api.Packet{
					Type: api.StreamMediaRequestPacket,
				}
				c.client.Send(pkt)
			}
		}
	}
}

// For non streaming clients (H3), trigger's end of call trigger for terminating underlying connection
func (c *riptClient) stop() {
	if !c.client.CanStream() {
		c.client.Close(nil)
	}
	c.stopChan <- true
	<-c.doneChan
}

func NewRIPTClient(client ript_net.Face, providerInfo *riptProviderInfo) *riptClient {
	hId, err := uuid.NewUUID()
	ad := "1 in: opus;\n" + "2 out: opus;\n"
	if err != nil {
		panic(err)
	}

	ript := &riptClient{
		client:   client,
		stopChan: make(chan bool, 1),
		doneChan: make(chan bool, 1),
		recvChan: make(chan api.PacketEvent, 1),
		handlerInfo: api.HandlerInfo{
			Id:            hId.String(),
			Advertisement: api.Advertisement(ad),
		},
		providerInfo: providerInfo,
	}

	ript.client.SetReceiveChan(ript.recvChan)

	return ript
}

func main() {
	portaudio.Initialize()
	defer portaudio.Terminate()

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var server string
	var xport string
	var mode string
	var dev bool

	flag.StringVar(&server, "server", "", "server url as fqdn")
	flag.StringVar(&xport, "xport", "", "type of transport (h3/ws)")
	flag.StringVar(&mode, "mode", "", "push or pull media")
	flag.BoolVar(&dev, "dev", false, "run client in dev mode with self-signed certs (needed for localhost)")
	flag.Parse()

	if server == "" {
		log.Printf("Missing server address.")
		return
	}

	if mode == "" {
		log.Printf("mode not specified. please specify push or pull")
		return
	}

	if xport == "" {
		log.Printf("xport not specified. please specify oneOf(h3/ws)")
		return
	}

	log.Printf("Server [%s], Mode [%s], Transport [%s]", server, mode, xport)

	var client ript_net.Face
	var err error
	provider := &riptProviderInfo{
		baseUrl: server,
	}
	if xport == "h3" {
		client = NewQuicClientFace(provider, dev)
	} else if xport == "ws" {
		client, err = ript_net.NewWebSocketClientFace("ws://localhost:8080/")
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("invalid xport type")
		return
	}

	riptClient := NewRIPTClient(client, provider)

	// 1. retrieve trunk groups
	riptClient.retrieveTrunkGroups()

	// 2. register this handler
	riptClient.registerHandler()

	// 3. create calls object
	riptClient.placeCalls()

	if mode == "push" {
		go riptClient.recordContent()
	}

	if mode == "pull" {
		go riptClient.playOutContent()
	}

	<-sigs
	riptClient.stop()
	log.Println("Done .. exiting now !!!")
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
