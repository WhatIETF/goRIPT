package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bifurcation/mint/syntax"

	"github.com/WhatIETF/goRIPT/common"

	"github.com/WhatIETF/goRIPT/api"
	"github.com/labstack/gommon/log"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

type QuicClientFace struct {
	client     *http.Client
	serverInfo *riptProviderInfo
	name       api.FaceName
	recvChan   chan api.PacketEvent
	haveRecv   bool
	sendChan   chan api.Packet
	closeChan  chan error
	haveClosed bool
}

func NewQuicClientFace(serverInfo *riptProviderInfo, dev bool) *QuicClientFace {

	pool, err := x509.SystemCertPool()
	if err != nil {
		fmt.Printf("cert pool creation error")
		return nil
	}
	// add ca-cert when run in dev mode alone
	if dev {
		common.AddRootCA(pool)
	}

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

	client := &http.Client{
		Transport: roundTripper,
		Timeout:   2 * time.Second,
	}

	url := serverInfo.baseUrl + "/media/join"
	log.Info("ript_client: registering to the server...[%s]", url)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 {
		fmt.Printf("ript_client: register failed. Status code %v", resp.StatusCode)
		return nil
	}

	log.Info("ript_client: register success !!!")

	return &QuicClientFace{
		client:     client,
		serverInfo: serverInfo,
		haveRecv:   false,
		haveClosed: false,
		closeChan:  make(chan error, 1),
	}
}

func (c *QuicClientFace) Name() api.FaceName {
	return api.FaceName(c.name)
}

func (c *QuicClientFace) CanStream() bool {
	return false
}

func (c *QuicClientFace) Read() {
	// ..... //
}

func (c *QuicClientFace) Send(pkt api.Packet) error {
	buf := new(bytes.Buffer)
	var err error

	err = json.NewEncoder(buf).Encode(pkt)
	if err != nil {
		log.Errorf("ript_client:send: marshal error")
		return err
	}

	// TODO: refactor the cases here.
	var res *http.Response
	var responsePacket api.Packet
	err = nil
	switch pkt.Type {
	case api.StreamMediaPacket:
		url := c.serverInfo.baseUrl + c.serverInfo.getTrunkGroupUri() + "/calls/123/media"
		log.Printf("ript_client: mediaPush: Url [%s]", url)

		enc, err := syntax.Marshal(pkt.StreamMedia)
		if err != nil {
			panic(err)
		}

		req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(enc))
		if err != nil {
			break
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		res, err = c.client.Do(req)
		if err != nil || res.StatusCode != 200 {
			break
		}
		log.Printf("ript_client:send: posted media fragment Id [%d], len [%d]", pkt.StreamMedia.SeqNo,
			len(pkt.StreamMedia.Media))

	case api.StreamMediaRequestPacket:
		url := c.serverInfo.baseUrl + c.serverInfo.getTrunkGroupUri() + "/calls/123/media"
		log.Printf("ript_client: mediaPull: Url [%s]", url)
		res, err = c.client.Get(url)
		if err != nil || res.StatusCode != 200 {
			break
		}

		// extract the binary encoded media payload
		body := &bytes.Buffer{}
		_, err := io.Copy(body, res.Body)
		if err != nil {
			log.Errorf("ript_client: error retrieving the body: [%v]", err)
			break
		}

		var media api.StreamContentMedia
		_, err = syntax.Unmarshal(body.Bytes(), &media)
		if err != nil {
			log.Errorf("ript_client: media payload unmarshal error [%v]", err)
			break
		}

		// construct pkt to forward it to the app layyer
		responsePacket := api.Packet{
			Type:        api.StreamMediaPacket,
			StreamMedia: media,
		}

		log.Printf("ript_client:mediapull: received content Id [%d], len [%d] bytes",
			responsePacket.StreamMedia.SeqNo, len(responsePacket.StreamMedia.Media))

		// forward the packet for further processing
		c.recvChan <- api.PacketEvent{
			Packet: responsePacket,
		}

	case api.RegisterHandlerPacket:
		url := c.serverInfo.baseUrl + c.serverInfo.getTrunkGroupUri() + "/handlers"
		res, err = c.client.Post(url, "application/json; charset=utf-8", buf)
		if err != nil || res.StatusCode != 200 {
			break
		}

		responsePacket, err = httpResponseToRiptPacket(res)
		if err != nil {
			break
		}

		log.Printf("ript_client: HandlerRegistration response [%v]", res)

		// forward the packet for further processing
		c.recvChan <- api.PacketEvent{
			Packet: responsePacket,
		}

	case api.CallsPacket:
		url := c.serverInfo.baseUrl + c.serverInfo.getTrunkGroupUri() + "/calls"
		res, err = c.client.Post(url, "application/json; charset=utf-8", buf)
		if err != nil || res.StatusCode != 200 {
			break
		}

		responsePacket, err = httpResponseToRiptPacket(res)
		if err != nil {
			break
		}

		log.Printf("ript_client: Calls response [%v]", res)

		// forward the packet for further processing
		c.recvChan <- api.PacketEvent{
			Packet: responsePacket,
		}

	case api.TrunkGroupDiscoveryPacket:
		trunkDiscoveryUrl := c.serverInfo.baseUrl + "/.well-known/ript/v1/providertgs"
		fmt.Printf("ript_client: trunkDiscovery url [%s]", trunkDiscoveryUrl)
		res, err = c.client.Get(trunkDiscoveryUrl)
		if err != nil || res.StatusCode != 200 {
			break
		}

		responsePacket, err = httpResponseToRiptPacket(res)
		if err != nil {
			break
		}

		log.Printf("TrunkGroupDiscoveryy response [%v]", res)

		// forward the packet for further processing
		c.recvChan <- api.PacketEvent{
			Packet: responsePacket,
		}
	}

	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("ript_client:send: failed status: [%v]", res.StatusCode)
	}

	res.Body.Close()
	return nil
}

func (c *QuicClientFace) SetReceiveChan(recv chan api.PacketEvent) {
	c.haveRecv = true
	c.recvChan = recv
}

func (c *QuicClientFace) Close(err error) {
	fmt.Printf("Close called on QuicFace with err %v\n", err)

	leaveUrl := c.serverInfo.baseUrl + "/media/leave"
	log.Info("ript_client: Unregistering from the server...")
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

///////
//// helpers
///////

func httpResponseToRiptPacket(response *http.Response) (api.Packet, error) {
	if response == nil {
		return api.Packet{}, errors.New("ript_client: invalid response object")
	}

	body := &bytes.Buffer{}
	_, err := io.Copy(body, response.Body)
	if err != nil {
		log.Errorf("ript_client: error retrieving the body: [%v]", err)
		return api.Packet{}, err
	}

	var packet api.Packet
	err = json.Unmarshal(body.Bytes(), &packet)
	if err != nil {
		log.Errorf("ript_client: content unmarshal [%v]", err)
		return api.Packet{}, err
	}

	return packet, nil
}
