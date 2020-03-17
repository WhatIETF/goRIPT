package ript_net

import (
	"fmt"
	"testing"
)

func TestWebSocketFace(t *testing.T) {
	forceInboundPrompt()

	port := 8080
	url := fmt.Sprintf("ws://localhost:%d/", port)

	server := NewWebSocketFaceServer(port)
	clientFace, err := NewWebSocketClientFace(url)
	if err != nil {
		t.Fatalf("Failed to create WebSocket client [%v]", err)
	}

	serverFace := <-server.Feed()
	faceTest(t, clientFace, serverFace)
}
