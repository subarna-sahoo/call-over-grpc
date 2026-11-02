// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/subarna-sahoo/call-over-grpc/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type Server struct {
	pb.UnimplementedCallServiceServer
	rooms map[string][]pb.CallService_JoinRoomServer
	// lock guards rooms and also serializes Send calls, since a gRPC stream
	// must not be written to from several goroutines at once.
	lock sync.Mutex
}

func NewServer() *Server {
	return &Server{rooms: make(map[string][]pb.CallService_JoinRoomServer)}
}

func (s *Server) JoinRoom(req *pb.JoinRequest, stream pb.CallService_JoinRoomServer) error {
	clientIP, _ := stream.Context().Value("client-ip").(string)
	userID := clientIP // using IP as userID

	s.lock.Lock()
	s.rooms[req.RoomId] = append(s.rooms[req.RoomId], stream)
	s.lock.Unlock()

	s.broadcast(req.RoomId, &pb.RoomEvent{
		UserId:    userID,
		EventType: "joined",
		Payload:   fmt.Sprintf("%s joined", userID),
	})

	// Keep stream open until the client goes away, then drop it from the room
	<-stream.Context().Done()
	s.leave(req.RoomId, stream)
	s.broadcast(req.RoomId, &pb.RoomEvent{
		UserId:    userID,
		EventType: "left",
		Payload:   fmt.Sprintf("%s left", userID),
	})
	return nil
}

func (s *Server) SendSignal(ctx context.Context, req *pb.SignalRequest) (*pb.SignalResponse, error) {
	s.broadcast(req.RoomId, &pb.RoomEvent{
		UserId:    req.FromUser,
		EventType: req.SignalType,
		Payload:   req.SignalData,
	})
	return &pb.SignalResponse{Status: "ok"}, nil
}

func (s *Server) broadcast(roomId string, event *pb.RoomEvent) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, client := range s.rooms[roomId] {
		if err := client.Send(event); err != nil {
			log.Printf("send to room %s failed: %v", roomId, err)
		}
	}
}

func (s *Server) leave(roomId string, stream pb.CallService_JoinRoomServer) {
	s.lock.Lock()
	defer s.lock.Unlock()
	clients := s.rooms[roomId]
	for i, client := range clients {
		if client == stream {
			clients = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(clients) == 0 {
		delete(s.rooms, roomId)
	} else {
		s.rooms[roomId] = clients
	}
}

// NewGRPCServer builds the gRPC server with the client-IP interceptors and the
// CallService registered.
func NewGRPCServer(svc *Server) *grpc.Server {
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			p, ok := peer.FromContext(ctx)
			if ok {
				ctx = context.WithValue(ctx, "client-ip", p.Addr.String())
			}
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			p, ok := peer.FromContext(ss.Context())
			if ok {
				ctx := context.WithValue(ss.Context(), "client-ip", p.Addr.String())
				ss = &wrappedStream{ServerStream: ss, ctx: ctx}
			}
			return handler(srv, ss)
		}),
	)
	pb.RegisterCallServiceServer(grpcServer, svc)
	return grpcServer
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := NewGRPCServer(NewServer())
	log.Println("gRPC server running on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
