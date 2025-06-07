// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	pb "video_signaling/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type Server struct {
	pb.UnimplementedCallServiceServer
	rooms map[string][]pb.CallService_JoinRoomServer
	lock  sync.RWMutex
}

func (s *Server) JoinRoom(req *pb.JoinRequest, stream pb.CallService_JoinRoomServer) error {
	clientIP := stream.Context().Value("client-ip").(string)
	userID := clientIP // using IP as userID

	s.lock.Lock()
	s.rooms[req.RoomId] = append(s.rooms[req.RoomId], stream)
	s.lock.Unlock()

	event := &pb.RoomEvent{
		UserId:    userID,
		EventType: "joined",
		Payload:   fmt.Sprintf("%s joined", userID),
	}
	s.broadcast(req.RoomId, event)

	// Keep stream open
	for {
		<-stream.Context().Done()
		break
	}
	return nil
}

func (s *Server) SendSignal(ctx context.Context, req *pb.SignalRequest) (*pb.SignalResponse, error) {
	s.lock.RLock()
	clients := s.rooms[req.RoomId]
	s.lock.RUnlock()

	event := &pb.RoomEvent{
		UserId:    req.FromUser,
		EventType: req.SignalType,
		Payload:   req.SignalData,
	}

	for _, client := range clients {
		client.Send(event)
	}
	return &pb.SignalResponse{Status: "ok"}, nil
}

func (s *Server) broadcast(roomId string, event *pb.RoomEvent) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for _, client := range s.rooms[roomId] {
		client.Send(event)
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

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

	pb.RegisterCallServiceServer(grpcServer, &Server{
		rooms: make(map[string][]pb.CallService_JoinRoomServer),
	})
	log.Println("gRPC server running on :50051")
	grpcServer.Serve(lis)
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
