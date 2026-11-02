package main

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/subarna-sahoo/call-over-grpc/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func startServer(t *testing.T) (pb.CallServiceClient, *Server) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	svc := NewServer()
	srv := NewGRPCServer(svc)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewCallServiceClient(conn), svc
}

func recvType(t *testing.T, stream pb.CallService_JoinRoomClient, want string) *pb.RoomEvent {
	t.Helper()
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if ev.EventType != want {
		t.Fatalf("got event %q, want %q", ev.EventType, want)
	}
	return ev
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func roomSize(svc *Server, room string) int {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	return len(svc.rooms[room])
}

func TestSignalReachesEveryoneInRoom(t *testing.T) {
	client, svc := startServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	alice, err := client.JoinRoom(ctx, &pb.JoinRequest{RoomId: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	recvType(t, alice, "joined") // her own join

	bob, err := client.JoinRoom(ctx, &pb.JoinRequest{RoomId: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	recvType(t, alice, "joined") // bob joined
	recvType(t, bob, "joined")
	waitFor(t, func() bool { return roomSize(svc, "r1") == 2 })

	resp, err := client.SendSignal(ctx, &pb.SignalRequest{RoomId: "r1", FromUser: "alice", SignalType: "offer", SignalData: "sdp"})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("SendSignal: %v %v", resp, err)
	}
	for _, s := range []pb.CallService_JoinRoomClient{alice, bob} {
		ev := recvType(t, s, "offer")
		if ev.UserId != "alice" || ev.Payload != "sdp" {
			t.Fatalf("unexpected event %+v", ev)
		}
	}
}

func TestDisconnectedClientIsRemoved(t *testing.T) {
	client, svc := startServer(t)
	ctx := context.Background()

	aliceCtx, aliceLeave := context.WithCancel(ctx)
	alice, _ := client.JoinRoom(aliceCtx, &pb.JoinRequest{RoomId: "r2"})
	recvType(t, alice, "joined")

	bob, _ := client.JoinRoom(ctx, &pb.JoinRequest{RoomId: "r2"})
	recvType(t, bob, "joined")
	waitFor(t, func() bool { return roomSize(svc, "r2") == 2 })

	aliceLeave()
	recvType(t, bob, "left")
	waitFor(t, func() bool { return roomSize(svc, "r2") == 1 })
}
