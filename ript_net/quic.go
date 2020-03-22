package ript_net

import (
	"bytes"
	"strconv"

	//"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/WhatIETF/goRIPT/api"
	//"github.com/caddyserver/certmagic"
	"github.com/gorilla/mux"
	"github.com/labstack/gommon/log"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"io"
	"net/http"
	"os"
	"time"
)

// quic based transport

type QuicFace struct {
	haveRecv  bool
	// inbound face to router for processsing
	recvChan chan api.PacketEvent
	// app/router to face for outbound transport/handlers
	contentChan chan api.Packet
	// channel for trunk discovery
	tgDiscChan chan api.Packet
	closeChan chan error
	closed    bool
	name string
}

func (f *QuicFace) handleClose(code int, text string) error {
	// todo implement
	return nil
}

func (f *QuicFace) Read() {
	// nothing to implement  unless we
}

func (f *QuicFace) Name() api.FaceName {
	return api.FaceName(f.name)
}


func (f *QuicFace) Send(pkt api.Packet) error {
	switch pkt.Type {
	case api.TrunkGroupDiscoveryPacket:
		log.Printf("send: passing on the content to trunk discovery chan, face [%s]",  f.name)
		f.tgDiscChan <- pkt
	default:
		log.Printf("send: passing on the content to general contnet chan, face [%s]",  f.name)
		f.contentChan <- pkt
	}
	return nil
}

func (f *QuicFace) SetReceiveChan(recv chan api.PacketEvent) {
	f.haveRecv = true
	f.recvChan = recv
}

func (f *QuicFace) Close(err error) {
	// todo: implement this
	fmt.Printf("Close called on QuicFace with err %v\n", err)
}

func (f *QuicFace) OnClose() chan error {
	return f.closeChan
}

func (f *QuicFace) CanStream() bool {
	return false
}

func NewQuicFace(name string) *QuicFace {
	q := &QuicFace {
		haveRecv:  false,
		closeChan: make(chan error, 1),
		contentChan: make(chan api.Packet, 1),
		tgDiscChan: make(chan api.Packet, 1),
		closed:    false,
		name:  name,
	}
	fmt.Printf("NewQuicFace %s created\n", name)
	return q
}


///////
// Server
type QuicFaceServer struct {
	*http3.Server
	feedChan chan Face
	// this is needed until we figure out a way to get triggered
	// by the connection creation (see todo)
	faceMap map[string]*QuicFace
}


func HandleMediaPull(face *QuicFace, writer http.ResponseWriter, request *http.Request) {
	// await media packet
	select {
	case resPkt := <- face.contentChan:
		log.Printf("mediaPull [%s] got content Id [%d] , len [%d] to send out", face.Name(),
			resPkt.Content.Id, len(resPkt.Content.Content))
		enc, err := json.Marshal(resPkt)
		if err != nil {
			writer.WriteHeader(400)
			return
		}
		writer.Write(enc)
	}
}


func HandleMediaPush(face *QuicFace, writer http.ResponseWriter, request *http.Request) {
	// 1. let the router's recv chan know of the pull request
	// 2. await response from the router
	log.Info("mediaPush: got packet\n")
	body := &bytes.Buffer{}
	_, err := io.Copy(body, request.Body)
	if err != nil {
		log.Errorf("Error retrieving the body: [%v]", err)
		writer.WriteHeader(400)
		return
	}
	var pkt api.Packet
	err = json.Unmarshal(body.Bytes(), &pkt)
	if err != nil {
		log.Printf("Error unmarshal [%v]", err)
		writer.WriteHeader(400)
		return
	}
	face.recvChan <- api.PacketEvent{
		Sender: face.Name(),
		Packet: pkt,
	}
	writer.WriteHeader(200)
}

func HandlerRegistration(face *QuicFace, writer http.ResponseWriter, request *http.Request) {
	log.Printf("Handler registration from [%v]", request)
	// extract trunkGroupId
	params := mux.Vars(request)
	tgId := params["trunkGroupId"]
	if len(tgId) == 0 {
		log.Errorf("missing trunkGroupId")
		writer.WriteHeader(400)
		return
	}

	// extract handler info from the body
	body := &bytes.Buffer{}
	_, err := io.Copy(body, request.Body)
	if err != nil {
		log.Errorf("Error retrieving the body: [%v]", err)
		writer.WriteHeader(400)
		return
	}
	var pkt api.Packet
	err = json.Unmarshal(body.Bytes(), &pkt)
	if err != nil {
		log.Printf("Error unmarshal [%v]", err)
		writer.WriteHeader(400)
		return
	}

	log.Printf("HandlerRegistration: trunk [%s], Request [%v]", tgId, pkt)

	// pass the packet to router
	face.recvChan <- api.PacketEvent{
		Sender: face.Name(),
		Packet: pkt,
	}

	// await response or timeout
	select {
	case <-time.After(2 * time.Second):
		log.Errorf("handlerRegistration: no content received .. ")
		writer.WriteHeader(404)
		return
	case resPkt := <- face.contentChan:
		log.Printf("handlerRegistration [%s] got content [%v]", face.Name(), resPkt)
		enc, err := json.Marshal(resPkt)
		if err != nil {
			writer.WriteHeader(400)
			return
		}
		writer.Write(enc)
	}
}


func HandleTgDiscovery(face *QuicFace, writer http.ResponseWriter, request *http.Request) {
	// query service for list of trunk groups available
	face.recvChan <- api.PacketEvent{
		Sender: face.Name(),
		Packet: api.Packet{
			Type: api.TrunkGroupDiscoveryPacket,
		},
	}

	// await response or timeout
	select {
	case <-time.After(2 * time.Second):
		log.Errorf("HandleTgDiscovery: no content received .. ")
		writer.WriteHeader(404)
		return
	case resPkt := <- face.tgDiscChan:
		log.Printf("HandleTgDiscovery [%s] got content [%v]", face.Name(), resPkt)
		enc, err := json.Marshal(resPkt)
		if err != nil {
			writer.WriteHeader(400)
			return
		}
		writer.Write(enc)
	}
}

// Mux handler for routing various h3 endpoints
func setupHandler(server *QuicFaceServer) http.Handler {
	router := mux.NewRouter()

	mediaPullFn := func(w http.ResponseWriter, r *http.Request) {
		//  get the face
		face := server.faceMap[r.RemoteAddr]
		log.Printf("mediaPull: Face [%s]", face.Name())
		HandleMediaPull(face, w, r)
	}

	mediaPushFn := func(w http.ResponseWriter, r *http.Request) {
		//  get the face
		face := server.faceMap[r.RemoteAddr]
		log.Printf("mediaPush: Face [%s]", face.Name())
		HandleMediaPush(face, w, r)
	}

	// trigger's face creation as well
	joinFn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Join from  [%v]", r.RemoteAddr)
		face := NewQuicFace(r.RemoteAddr)
		server.feedChan <- face
		server.faceMap[r.RemoteAddr] = face
		w.WriteHeader(200)
	}

	leaveFn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Leave from  [%v]", r.RemoteAddr)
		//  get the face
		face := server.faceMap[r.RemoteAddr]
		if face != nil {
			face.closeChan <- errors.New("client leave")
			close(face.contentChan)
			delete(server.faceMap, r.RemoteAddr)
		}
		w.WriteHeader(200)
	}

	regHandlerFn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("register handler from  [%v]", r.RemoteAddr)
		//  get the face
		face := server.faceMap[r.RemoteAddr]
		HandlerRegistration(face, w, r)
	}


	tgDiscFn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("trunk group discovery  from  [%v]", r.RemoteAddr)
		//  get the face
		face := server.faceMap[r.RemoteAddr]
		HandleTgDiscovery(face, w, r)
	}


	fmt.Printf("trunkDiscoveryUrl [%s]", baseUrl+"/providerTgs")
	router.HandleFunc(baseUrl+"/providertgs", tgDiscFn)

	// handler registrations
	router.HandleFunc("/.well-known/ript/v1/providertgs/{trunkGroupId}/handlers",
		regHandlerFn).Methods(http.MethodPost)

	router.HandleFunc("/media/join", joinFn)
	router.HandleFunc("/media/leave", leaveFn)

	// media byways
	router.HandleFunc("/media/forward", mediaPushFn).Methods(http.MethodPut)
	router.HandleFunc("/media/reverse", mediaPullFn).Methods(http.MethodGet)

	return router
}


func NewQuicFaceServer(port int, host, certFile, keyFile string) *QuicFaceServer {
	url := host + ":" + strconv.Itoa(port)
	log.Printf("Server Url [%s]", url)

	// rest of the config seems sane
	quicConf := &quic.Config{
		KeepAlive: true,
	}

	quicConf.GetLogWriter = func(connID []byte) io.WriteCloser {
		filename := fmt.Sprintf("server_%x.qlog", connID)
		f, err := os.Create(filename)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Creating qlog file %s.\n", filename)
		return f
	}

	quicServer := &QuicFaceServer{
		Server: &http3.Server{
			Server:     &http.Server{Handler: nil, Addr: url},
			QuicConfig: quicConf,
		},
		feedChan: make(chan Face, 10),
		faceMap: map[string]*QuicFace{},
	}
	handler := setupHandler(quicServer)
	quicServer.Handler = handler

	log.Printf("Starting Server certFile [%s], keyFile [%s]", certFile, keyFile)

	go quicServer.ListenAndServeTLS(certFile, keyFile)
	//if err != nil {
	//	log.Fatalf("server start error [%v]", err)
	//}


	log.Info("New QUIC-H3 Server created.\n")
	return quicServer
}


func (server *QuicFaceServer) Feed() chan Face {
	return server.feedChan
}
