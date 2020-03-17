package main

import (
	"flag"
	"fmt"
	"github.com/WhatIETF/goRIPT/ript_net"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 6121, "port on which to listen")
	flag.Parse()

	router := ript_net.NewRouter("ript-relay")

	// h3 Server
	h3Server := ript_net.NewQuicFaceServer(6121)
	router.AddFaceFactory(h3Server)

	// ws Server
	wsServer := ript_net.NewWebSocketFaceServer(8080)
	router.AddFaceFactory(wsServer)

	fmt.Printf("Router started and serving on port %d\n", port)
	select {}
}