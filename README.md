# call-over-grpc

A **gRPC signaling server** in Go for video and voice calls. Clients join a room over a server-streaming RPC and exchange signaling messages (such as WebRTC offers, answers and ICE candidates). The server fans each message out to everyone in the room.

## How it works

| RPC | Type | Description |
|---|---|---|
| `JoinRoom(JoinRequest)` | server stream | Adds the caller to a room and streams `RoomEvent`s to them. Everyone already in the room gets a `joined` event. |
| `SendSignal(SignalRequest)` | unary | Broadcasts `{from_user, signal_type, signal_data}` to every client in the room. |

- Rooms are kept in memory as `map[roomId][]stream`, guarded by a `sync.RWMutex`.
- Unary and stream interceptors attach the client's IP address to the request context. The IP is currently used as the user ID.
- The server listens on **`:50051`**.

## Running

> **Note:** `main.go` imports the generated protobuf package `video_signaling/proto`, which is not committed yet. Add the `.proto` file and generate the Go code before building.

```bash
# dependencies
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get github.com/improbable-eng/grpc-web/go/grpcweb   # for browser clients via gRPC-Web

# generate stubs (once the .proto is added)
protoc --go_out=. --go-grpc_out=. proto/*.proto

go run main.go
```

## Tech stack

Go 1.24 · gRPC · Protocol Buffers · gRPC-Web

## License

MIT. See [LICENSE](LICENSE).
