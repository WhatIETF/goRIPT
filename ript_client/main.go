package main

import (
	"flag"
	"fmt"
	"github.com/WhatIETF/goRIPT/api"
	"github.com/WhatIETF/goRIPT/ript_net"
	"github.com/google/uuid"
	"github.com/gordonklaus/portaudio"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// info about the provider
type riptProviderInfo struct {
	baseUrl string
	trunkGroups map[string][]api.TrunkGroup
}

func (p *riptProviderInfo) getTrunkGroupUri() string {
	// pick the outbound url as default
	tg := p.trunkGroups["outbound"][0]
	return tg.Uri
}

type riptClient struct {
	client ript_net.Face
	stopChan chan bool
	doneChan chan bool
	recvChan chan api.PacketEvent
	// ript semantics
	handlerInfo api.HandlerInfo
	providerInfo *riptProviderInfo
}

func (c *riptClient) registerHandler() {
	pkt := api.Packet{
		Type: api.RegisterHandlerPacket,
		RegisterHandler: api.RegisterHandlerMessage{
			HandlerRequest: api.HandlerRequest{
				HandlerId: c.handlerInfo.Id,
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
	case response := <- c.recvChan:
		c.handlerInfo.Uri = response.Packet.RegisterHandler.HandlerResponse.Uri
	}

	log.Printf("registerHandler: handlerInfo with uri: [%v]", c.handlerInfo)
}

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
	case response := <- c.recvChan:
		c.providerInfo.trunkGroups = response.Packet.TrunkGroupsInfo.TrunkGroups
	}

	log.Printf("trukGroupDiscovery: tgs [%v]", c.providerInfo.trunkGroups)
}

func (c *riptClient) recordContent(client ript_net.Face) {
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
			pkt := api.Packet{
				Type: api.ContentPacket,
				Filter: api.ContentFilterMediaForward,
				Content: api.ContentMessage{
					Id:      contentId,
					To:      "trunk123",
					Content: content,
				},
			}
			err := client.Send(pkt)
			if err != nil {
				log.Fatalf("recordContent: media send error [%v]", err)
				continue
			}
			contentId++
		}
	}
}

func (c * riptClient) playOutContent(client ript_net.Face) {
	defer func() {
		c.doneChan <- true
	}()

	c.client.SetReceiveChan(c.recvChan)
	speaker, err := NewSpeaker()
	chk(err)

	for {
		select {
		case <- c.stopChan:
			log.Println("playout stopped")
			return
		case evt := <- c.recvChan:
			log.Printf("got media evt : [%v]", evt)
			speaker.Play(evt.Packet.Content.Content)
			continue
		default:
			if !c.client.CanStream() {
				// ask server for the packet
				pkt := api.Packet{
					Type: api.ContentPacket,
					Filter: api.ContentFilterMediaReverse,
				}
				client.Send(pkt)
			}
		}
	}
}

func (c *riptClient) stop() {
	if !c.client.CanStream() {
		c.client.Close(nil)
	}
	c.stopChan <- true
	<- c.doneChan
}


func NewRIPTClient(client ript_net.Face, providerInfo *riptProviderInfo) *riptClient {
	hId, err := uuid.NewUUID()
	ad := "1 in: opus; PCMU; PCMA; 2 out: opus; PCMU; PCMA;"
	if err != nil {
		panic(err)
	}

	ript := &riptClient{
		client: client,
		stopChan:   make(chan bool, 1),
		doneChan:   make(chan bool, 1),
		recvChan: make(chan api.PacketEvent, 1),
		handlerInfo: api.HandlerInfo{
			Id: hId.String(),
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

	var xport string
	var textMode bool
	var mode string
	flag.StringVar(&xport, "xport", "", "type of transport (h3/ws/..)")
	flag.BoolVar(&textMode, "text", false, "Enable text-chat mode")
	flag.StringVar(&mode, "mode", "", "push or pull media")
	flag.Parse()


	if mode == "" {
		fmt.Printf("mode not specified. please specify push or pull")
		return
	}

	if xport == "" {
		fmt.Printf("xport not specified. please specify oneOf(h3/ws)")
		return
	}

	var client ript_net.Face
	var err error
	provider := &riptProviderInfo{
		baseUrl: baseUri,
	}
	if xport == "h3" {
		client = NewQuicClientFace(provider)
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

	if mode == "push" {
		go riptClient.recordContent(client)
	}

	if mode == "pull" {
		go riptClient.playOutContent(client)
	}

	<- sigs
	riptClient.stop()
	log.Println("Done .. exiting now !!!")
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

