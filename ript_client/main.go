package main

import (
	"flag"
	"fmt"
	"github.com/gordonklaus/portaudio"
	"github.com/WhatIETF/goRIPT/ript_net"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type riptClient struct {
	client ript_net.Face
	stopChan   chan bool
	doneChan chan bool
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
			pkt := ript_net.Packet{
				Type: ript_net.ContentPacket,
				Content: ript_net.ContentMessage{
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

	recvChan := make(chan ript_net.PacketEvent, 1)
	client.SetReceiveChan(recvChan)
	speaker, err := NewSpeaker()
	chk(err)

	for {
		select {
		case <- c.stopChan:
			log.Println("playout stopped")
			return
		case evt := <- recvChan:
			log.Printf("got media evt : [%v]", evt)
			speaker.Play(evt.Packet.Content.Content)
			continue
		default:
			if !client.CanStream() {
				client.Read()
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


func NewRIPTClient(client ript_net.Face) *riptClient {
	return &riptClient{
		client: client,
		stopChan:   make(chan bool, 1),
		doneChan:   make(chan bool, 1),
	}
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
	if xport == "h3" {
		client = NewQuicClientFace()
	} else if xport == "ws" {
		client, err = ript_net.NewWebSocketClientFace("ws://localhost:8080/")
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("invalid xport type")
		return
	}

	riptClient := NewRIPTClient(client)
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

