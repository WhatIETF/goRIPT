package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"crypto/tls"
	"crypto/x509"
	"github.com/labstack/gommon/log"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/WhatIETF/goRIPT/common"
	"github.com/WhatIETF/goRIPT/ript_net"
	"io"
	"net/http"
	"time"
)

type QuicClientFace struct {
	client *http.Client
	name ript_net.FaceName
	recvChan chan ript_net.PacketEvent
	haveRecv bool
	sendChan chan ript_net.Packet
	closeChan chan error
	haveClosed bool
	inboundContentId int32
}

func NewQuicClientFace() *QuicClientFace {
	pool, err := x509.SystemCertPool()
	if err != nil {
		fmt.Printf("cert pool creation error")
		return nil
	}

	common.AddRootCA(pool)

	quicConf := &quic.Config{
		KeepAlive: true,
	}

	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			RootCAs:            pool,
			InsecureSkipVerify: false,
		},
		QuicConfig: quicConf,
	}

	client := &http.Client {
		Transport: roundTripper,
		Timeout: 2 * time.Second,
	}

	registerUrl := "https://localhost:6121/media/join"
	log.Info("ript_client: registering to the server...")
	resp, err := client.Get(registerUrl)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 {
		fmt.Printf("ript_client: register failed. Status code %v", resp.StatusCode)
		return nil
	}

	log.Info("ript_client: register success !!!")

	return &QuicClientFace {
		client: client,
		haveRecv: false,
		haveClosed: false,
		closeChan: make(chan error, 1),
		inboundContentId: -1,

	}
}

func (c *QuicClientFace) Name() ript_net.FaceName {
	return ript_net.FaceName(c.name)
}

func (c *QuicClientFace) CanStream() bool {
	return false
}

func (c *QuicClientFace) Read() {
	// read the packet from the remote end and pass on the received packet
	// to the channel that process it
	mediaPullUrl := "https://localhost:6121/media/reverse"
	log.Printf("ript_client:read: requesting content for Id [%d]", c.inboundContentId)
	req := ript_net.Packet{
		Type: ript_net.ContentRequestPacket,
		ContentRequest: ript_net.ContentRequestMessage{
			To: "trunk123",
			Id: c.inboundContentId,
		},
	}

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(req)
	if err != nil {
		log.Errorf("ript_client:read: marshal error [%v]", err)
		// todo: don't panic and report error for app to handle
		panic(err)
	}

	res, err := c.client.Post(mediaPullUrl, "application/json; charset=utf-8", buf)
	if err != nil {
		log.Errorf("ript_client:read: media pull error [%v]", err)
		return
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Errorf("ript_client:read: non 200 response [%d]", res.StatusCode)
		// todo: retry the request if a given id is not found .. slow sender perhaps ?
		return
	}

	body := &bytes.Buffer{}
	_, err = io.Copy(body, res.Body)
	if err != nil {
		log.Errorf("ript_client:read: error retrieving the body: [%v]", err)
		// todo: don't panic and report error for app to handle
		panic(err)
	}

	var mediaPacket ript_net.Packet
	err = json.Unmarshal(body.Bytes(), &mediaPacket)
	if err != nil {
		log.Errorf("ript_client: content unmarshal [%v]", err)
		// todo: don't panic and report error for app to handle
		panic(err)
	}

	log.Printf("ript_client:read: received content Id [%d], len [%d] bytes",
		mediaPacket.Content.Id, len(mediaPacket.Content.Content))

	c.recvChan <- ript_net.PacketEvent{
		Packet: mediaPacket,
	}

	c.inboundContentId = mediaPacket.Content.Id + 1
}


func (c *QuicClientFace) Send(pkt ript_net.Packet) error {
	mediaPushUrl := "https://localhost:6121/media/forward"
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(pkt)
	if err != nil {
		log.Errorf("ript_client:send: marshal error")
		return err
	}

	res, err := c.client.Post(mediaPushUrl, "application/json; charset=utf-8", buf)
	if err != nil {
		log.Errorf("ript_client:send: POST error [%v]\n", err)
		panic(err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("ript_client:send: post media id [%d] failed [%v]", pkt.Content.Id, res)
	}
	log.Printf("ript_client:send: posted media fragment Id [%d], len [%d]", pkt.Content.Id, len(pkt.Content.Content))
	return nil
}

func (c *QuicClientFace) SetReceiveChan(recv chan ript_net.PacketEvent) {
	c.haveRecv = true
	c.recvChan = recv
}

func (c *QuicClientFace) Close(err error) {
	fmt.Printf("Close called on QuicFace with err %v\n", err)

	leaveUrl := "https://localhost:6121/media/leave"
	log.Info("ript_client: registering to the server...")
	resp, err := c.client.Get(leaveUrl)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 {
		fmt.Printf("ript_client: leave failed. Status code %v", resp.StatusCode)
	}

}

func (c *QuicClientFace) OnClose() chan error {
	return c.closeChan
}
