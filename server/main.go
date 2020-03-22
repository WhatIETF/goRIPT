package main

import (
	"flag"
	"fmt"
	"github.com/WhatIETF/goRIPT/ript_net"
)

// TODO: Move config handling into a utility
func main() {
	var h3Port int
	var wssPort int
	var serverHost string
	var certFile string
	var keyFile string

	flag.StringVar(&serverHost, "host", "", "server address.")
	flag.IntVar(&h3Port, "h3port", 2399, "H3 port on which to listen")
	flag.IntVar(&wssPort, "wssport", 8080, "WSS port on which to listen")
	flag.StringVar(&certFile,"certfile", "", "Full path for server cert file")
	flag.StringVar(&keyFile,"keyfile", "", "Full path for server key file")

	flag.Parse()

	// validate arguments
	if certFile == "" {
		certFile = "/etc/letsencrypt/live/ietf107.ript-dev.com/fullchain.pem"
		fmt.Printf("Using Cert file %s\n", certFile)
	}

	if keyFile == "" {
		keyFile = "/etc/letsencrypt/live/ietf107.ript-dev.com/privkey.pem"
		fmt.Printf("Using KeyFile file %s\n", keyFile)
	}

	fmt.Printf("Host: %s, H3Port %d, WSSPort %d\n", serverHost, h3Port, wssPort)

	service := ript_net.NewRIPTService()
	router := ript_net.NewRouter("ript-relay", service)

	// h3 Server
	h3Server := ript_net.NewQuicFaceServer(h3Port, serverHost, certFile, keyFile)
	router.AddFaceFactory(h3Server)

	// ws Server
	wsServer := ript_net.NewWebSocketFaceServer(wssPort)
	router.AddFaceFactory(wsServer)

	fmt.Println("Router is ready to serve ...")
	select {}
}