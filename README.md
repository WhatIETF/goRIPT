# GoRIPT

WIP and in hack mode (except few landmines and no-so optimized code)

# Components

## Server

- Media router (ript_net) and server.go (driver program)
- Implements websockets and h3/quic transports
- Distributes media from sender to all the receivers

## Client

- Support WS and h3/quic transport (selected via command line flag)
- Implements unidirectional media (sender -> reciever(s))
   
Note: At present, self-signed certs are used to quic tls setup.

Note: Current code shows how h3 POST can be used to send/recv media for h3 transport.

# Build and Run

## Prerequisites

1. Install go 1.14 or later

2. pkg-config, portaudio, opus, opusfile

On OSX you might work 
```
brew install pkg-config portaudio  opus  opusfile
```

3. GO packages - install with

```
go get github.com/lucas-clemente/quic-go
go get github.com/gordonklaus/portaudio
go get gopkg.in/hraban/opus.v2
```

4. you will need this package in your gopath so it is found

## Run server:

```
go run server/main.go

The program takes few optional arguments

 -certfile string
    	Full path for server cert file
  -h3port int
    	H3 port on which to listen (default 2399)
  -host string
    	server address. (default "")
  -keyfile string
    	Full path for server key file
  -wssport int
    	WSS port on which to listen (default 8080)
    	
 For running locally certfile from (common/cert.pem) and keyfile(common/priv.key) 
 will be used.   	
```

## Run Clients

```
cd ript_client
go build .

 When running locally CA certfile form (commom/ca.pem) will be used by default. 
```

### H3 Sender -> H3 Receiver

Receiver:
```
./ript_client --server=https://localhost:2399 --mode=pull  --xport=h3 --dev
```

Sender:
```
./ript_client --server=https://localhost:2399  --mode=push --xport=h3 --dev
```
Note: "--dev" option is needed when clients are talking to server run locally.

