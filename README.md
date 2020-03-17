# http-media
WIP and in hack mode (except few landmines and no-so optimized code)

Components
----------

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

1. quic-go (go get tgithub.com/lucas-clemente/quic-go)


## Run server:
   ``` go run server/main.go ```

## Run Client (s)
```
cd ript_client
go build .
```

### H3 Sender -> H3 Receiver
```
./ript_client --mode=pull  --xport=h3 (receiver)

./ript_client --mode=push --xport=h3 (sender)
```

### WS Sender -> H3 Receiver
```
./ript_client --mode=pull  --xport=h3 (receiver)

./ript_client --mode=push --xport=ws (sender)
```



