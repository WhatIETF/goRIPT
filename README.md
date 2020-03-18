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

1. Intal go 1.14 or later

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
```

## Run Clients

```
cd ript_client
go build .
```

### H3 Sender -> H3 Receiver

Receiver:
```
./ript_client --mode=pull  --xport=h3 
```

Sender:
```
./ript_client --mode=push --xport=h3
```

### WS Sender -> H3 Receiver

Reciever: 
```
./ript_client --mode=pull  --xport=h3
```

Sender:
```
./ript_client --mode=push --xport=ws 
```



