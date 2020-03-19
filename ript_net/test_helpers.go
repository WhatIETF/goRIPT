package ript_net

import (
	"github.com/WhatIETF/goRIPT/api"
	"net"
	"reflect"
	"testing"
	"time"
)

func forceInboundPrompt() {
	in, _ := net.Listen("tcp", ":8081")
	go func() {
		conn, _ := net.Dial("tcp", "localhost:8081")
		conn.Write([]byte("hello"))
	}()
	conn, _ := in.Accept()
	buf := make([]byte, 10)
	conn.Read(buf)
}

func testFaceSend(t *testing.T, a, b Face) {
	pkt := api.Packet{
		Content: api.ContentMessage{
			Content: []byte("test"),
		},
	}

	recvB := make(chan api.PacketEvent, 1)
	b.SetReceiveChan(recvB)

	err := a.Send(pkt)
	if err != nil {
		t.Fatalf("Error on send [%v]", err)
	}

	// Brief pause for transmission
	<-time.After(100 * time.Millisecond)

	select {
	case evt := <-recvB:
		if evt.Sender != b.Name() {
			t.Fatalf("Incorrect sender in packet event")
		}
		if !reflect.DeepEqual(evt.Packet, pkt) {
			t.Fatalf("Incorrect packet in packet event")
		}

	case err := <-a.OnClose():
		t.Fatalf("Sender unexpectedly closed [%v]", err)

	case err := <-b.OnClose():
		t.Fatalf("Receiver unexpectedly closed [%v]", err)

	default:
		t.Fatalf("Unexpected blocking in send")
	}
}

// Faces "a" and "b" should be connected to each other
func faceTest(t *testing.T, a, b Face) {
	// Test that the names are non-empty
	if len(a.Name()) == 0 || len(b.Name()) == 0 {
		t.Fatalf("Face has empty name")
	}

	// Verify that packets can be sent a->b and b->a
	testFaceSend(t, a, b)
	//testFaceSend(t, b, a)

	// Verify that the OnClose event fires on being closed
	// (Assumes that the OnClose chan is buffered)
	a.Close(nil)
	select {
	case <-a.OnClose():
	default:
		t.Fatalf("OnClose event did not fire")
	}
}