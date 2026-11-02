# call-over-grpc

A **gRPC signaling server** in Go for video and voice calls. Clients join a room over a server-streaming RPC and exchange signaling messages (such as WebRTC offers, answers and ICE candidates). The server fans each message out to everyone in the room.

## How it works

| RPC | Type | Description |
|---|---|---|
| `JoinRoom(JoinRequest)` | server stream | Adds the caller to a room and streams `RoomEvent`s to them. Everyone already in the room gets a `joined` event. |
| `SendSignal(SignalRequest)` | unary | Broadcasts `{from_user, signal_type, signal_data}` to every client in the room. |

- Rooms are kept in memory as `map[roomId][]stream`, guarded by a `sync.RWMutex`.
- Unary and stream interceptors attach the client's IP address to the request context. The IP is currently used as the user ID.
- Clients that disconnect are removed from the room, and the others get a `left` event.
- The server listens on **`:50051`**.

## Running

```bash
go run .            # gRPC server on :50051
```

Try it with [grpcurl](https://github.com/fullstorydev/grpcurl):

```bash
# terminal 1: join a room and watch events
grpcurl -plaintext -import-path proto -proto signaling.proto \
  -d '{"room_id": "demo"}' localhost:50051 signaling.CallService/JoinRoom

# terminal 2: send a signal to the room
grpcurl -plaintext -import-path proto -proto signaling.proto \
  -d '{"room_id": "demo", "from_user": "alice", "signal_type": "offer", "signal_data": "<sdp>"}' \
  localhost:50051 signaling.CallService/SendSignal
```

## Tests

```bash
go test -race ./...
```

The tests start the server in memory (bufconn) and check that signals reach everyone in a room, and that clients are removed and a `left` event is sent when they disconnect.

## Regenerating the gRPC code

The service is defined in [`proto/signaling.proto`](proto/signaling.proto). The generated `*.pb.go` files are committed. After editing the proto, regenerate them with [buf](https://buf.build):

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
buf generate
```

## Tech stack

Go 1.24 · gRPC · Protocol Buffers · buf

## License

MIT. See [LICENSE](LICENSE).
