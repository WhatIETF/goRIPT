package ript_net

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type WebSocketFace struct {
	conn      *websocket.Conn
	haveRecv  bool
	recvChan  chan PacketEvent
	closeChan chan error
	closed    bool
}

func NewWebSocketFace(conn *websocket.Conn) *WebSocketFace {
	ws := &WebSocketFace{
		conn:      conn,
		haveRecv:  false,
		closeChan: make(chan error, 1),
		closed:    false,
	}
	go ws.Read()
	return ws
}

func (ws *WebSocketFace) handleClose(code int, text string) error {
	log.Printf("[%s] Connection closed [%d] [%s]", ws.Name(), code, text)
	ws.closed = true
	ws.closeChan <- fmt.Errorf("WebSocket closed [%d] [%s]", code, text)
	return nil
}

func (ws *WebSocketFace) Read() {
	var err error
	log.Printf("read: ws closes ? [%v]", ws.closed)
	for !ws.closed {
		var msgType int
		var message []byte
		msgType, message, err = ws.conn.ReadMessage()
		if err != nil {
			log.Fatalf("ws read error [%v]", err)
			break
		}
		if msgType != websocket.TextMessage {
			log.Fatalf("ws msgType mimsmatch got [%v]", msgType)
			break
		}

		var pkt Packet
		err = json.Unmarshal(message, &pkt)
		if err != nil {
			break
		}

		log.Printf("ws:read: haveRecv [%v], pkt [%v]", ws.haveRecv, pkt)
		if !ws.haveRecv {
			continue
		}

		ws.recvChan <- PacketEvent{
			Sender: ws.Name(),
			Packet: pkt,
		}
	}

	if err != nil && !ws.closed {
		log.Printf("read error [%+v]", err)
		ws.Close(err)
	}
}

func (ws *WebSocketFace) Name() FaceName {
	return FaceName(ws.conn.RemoteAddr().String())
}

func (ws *WebSocketFace) Send(pkt Packet) error {
	if ws.closed {
		return fmt.Errorf("Cannot send on closed channel")
	}

	enc, err := json.Marshal(pkt)
	if err != nil {
		return err
	}
	return ws.conn.WriteMessage(websocket.TextMessage, enc)
}

func (ws *WebSocketFace) SetReceiveChan(recv chan PacketEvent) {
	ws.haveRecv = true
	ws.recvChan = recv
}

func (ws *WebSocketFace) Close(err error) {
	log.Printf("[%s] Closing [%v]", ws.Name(), err)

	closeCode := websocket.CloseNormalClosure
	closeInfo := "Normal closure"
	if err != nil {
		closeCode = websocket.CloseInternalServerErr
		closeInfo = err.Error()
	}

	closeMsg := websocket.FormatCloseMessage(closeCode, closeInfo)
	ws.conn.WriteMessage(websocket.CloseMessage, closeMsg)
	ws.closed = true
	ws.conn.Close()
	ws.closeChan <- err
}

func (ws *WebSocketFace) OnClose() chan error {
	return ws.closeChan
}

func (ws *WebSocketFace) CanStream() bool {
	return true
}


/////

func NewWebSocketClientFace(url string) (*WebSocketFace, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return NewWebSocketFace(conn), nil
}

type WebSocketFaceServer struct {
	*http.Server
	recvChan chan PacketEvent
	feedChan chan Face
}

func NewWebSocketFaceServer(port int) *WebSocketFaceServer {
	wss := &WebSocketFaceServer{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
		feedChan: make(chan Face, 10),
	}

	wss.Handler = wss
	go wss.ListenAndServe()
	return wss
}

func (wss *WebSocketFaceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// TODO note the error somehow
		return
	}

	wss.feedChan <- NewWebSocketFace(conn)
}

func (wss *WebSocketFaceServer) Feed() chan Face {
	return wss.feedChan
}
