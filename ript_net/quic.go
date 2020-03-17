package ript_net

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/gommon/log"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/WhatIETF/goRIPT/common"
	"io"
	"net/http"
)

// quic based transport

type QuicFace struct {
	haveRecv  bool
	// inbound face to router for processsing
	recvChan chan PacketEvent
	// app/router to face for outbound transport
	contentChan chan Packet
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

func (f *QuicFace) Name() FaceName {
	return FaceName(f.name)
}


func (f *QuicFace) Send(pkt Packet) error {
	log.Printf("send: passing on the content [%d] to content chan, face [%s]", pkt.Content.Id, f.name)
	f.contentChan <- pkt
	return nil
}

func (f *QuicFace) SetReceiveChan(recv chan PacketEvent) {
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
		contentChan: make(chan Packet, 1),
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
	// 1. let the router's recv chan know of the pull request
	// 2. await response from the router
	/*
	body := &bytes.Buffer{}
	_, err := io.Copy(body, request.Body)
	if err != nil {
		log.Errorf("Error retrieving the body: [%v]", err)
		writer.WriteHeader(400)
		return
	}
	var pkt Packet
	err = json.Unmarshal(body.Bytes(), &pkt)
	if err != nil {
		log.Errorf("Error unmarshal [%v]", err)
		writer.WriteHeader(400)
		return
	}
	log.Printf("mediaPull: forwarding [%v] to router", pkt)
	face.recvChan <- PacketEvent{
		Sender: face.Name(),
		Packet: pkt,
	}
	*/

	select {
	/*case <-time.After(250 * time.Millisecond):
		log.Errorf("mediaPull no content received .. ")
		writer.WriteHeader(404)
		return*/
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
	var pkt Packet
	err = json.Unmarshal(body.Bytes(), &pkt)
	if err != nil {
		log.Printf("Error unmarshal [%v]", err)
		writer.WriteHeader(400)
		return
	}
	face.recvChan <- PacketEvent{
		Sender: face.Name(),
		Packet: pkt,
	}
	writer.WriteHeader(200)
}

// Mux handler for routing various h3 endpoints
func setupHandler(server *QuicFaceServer) http.Handler {
	mux := http.NewServeMux()

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
		return
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
		return
	}


	mux.HandleFunc("/media/forward", mediaPushFn)
	mux.HandleFunc("/media/reverse", mediaPullFn)
	mux.HandleFunc("/media/join", joinFn)
	mux.HandleFunc("/media/leave", leaveFn)

	return mux
}

func NewQuicFaceServer(port int) *QuicFaceServer {
	// rest of the config seems sane
	quicConf := &quic.Config{
		KeepAlive: true,
	}

	url := fmt.Sprintf("localhost:%d", port)
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
	go quicServer.ListenAndServeTLS(common.GetCertificatePaths())
	log.Info("New Quic Server created.\n")
	return quicServer
}


func (server *QuicFaceServer) Feed() chan Face {
	return server.feedChan
}
