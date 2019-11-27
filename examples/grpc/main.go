package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/yaegaki/hibari"
	hgrpc "github.com/yaegaki/hibari/grpc"
	pb "github.com/yaegaki/hibari/grpc/pb"
	"google.golang.org/grpc"
)

func client(index int) {
	<-time.After(time.Second)
	conn, err := grpc.Dial(":3311", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	c := pb.NewHibariClient(conn)
	stream, err := c.Conn(context.Background())
	if err != nil {
		panic(err)
	}
	defer stream.CloseSend()

	joinMsgBody := &pb.JoinMessageBody{
		UserID: fmt.Sprintf("u_%v", index),
		Secret: "a",
		RoomID: "a",
	}
	pbJoinMsgBody, err := ptypes.MarshalAny(joinMsgBody)
	if err != nil {
		panic(err)
	}

	err = stream.Send(&pb.Message{
		Kind: pb.Message_Join,
		Body: pbJoinMsgBody,
	})
	if err != nil {
		panic(err)
	}

	broadcastBody, _ := ptypes.MarshalAny(&pb.ShortUser{
		Index: 99,
		Name:  "broadcast-message-sample",
	})

	broadcast, _ := ptypes.MarshalAny(&pb.BroadcastMessageBody{
		Body: broadcastBody,
	})

	stream.Send(&pb.Message{
		Kind: pb.Message_Broadcast,
		Body: broadcast,
	})
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Print(err)
			break
		}

		log.Print(msg)
	}
}

func main() {
	go func() {
		for i := 0; i < 10; i++ {
			<-time.After(time.Second * 5)
			go client(i)
		}
	}()

	l, err := net.Listen("tcp", ":3311")
	if err != nil {
		panic(err)
	}

	manager := hibari.NewManager(roomAllocator{}, &hibari.ManagerOption{
		Authenticator: authenticator{
			mu: &sync.Mutex{},
		},
	})

	s := grpc.NewServer()
	pb.RegisterHibariServer(s, hgrpc.NewHibariServer(manager, hgrpc.HibariServerOption{}))
	s.Serve(l)
}
