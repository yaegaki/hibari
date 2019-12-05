package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/yaegaki/hibari"
	pb "github.com/yaegaki/hibari/examples/interop/pb"
	hgrpc "github.com/yaegaki/hibari/grpc"
	hpb "github.com/yaegaki/hibari/grpc/pb"
	"github.com/yaegaki/hibari/websocket"
	"google.golang.org/grpc"
)

func startGrpcClient() {
	conn, err := grpc.Dial(":3311", grpc.WithInsecure())
	if err != nil {
		log.Print(err)
		return
	}
	defer conn.Close()

	c := hpb.NewHibariClient(conn)
	stream, err := c.Conn(context.Background())
	if err != nil {
		log.Print(err)
		return
	}
	defer stream.CloseSend()

	joinMsgBody := &hpb.JoinMessageBody{
		UserID: "grpcuser",
		Secret: "",
		RoomID: "interop",
	}
	pbJoinMsgBody, err := ptypes.MarshalAny(joinMsgBody)
	if err != nil {
		panic(err)
	}

	err = stream.Send(&hpb.Message{
		Kind: hpb.Message_Join,
		Body: pbJoinMsgBody,
	})
	if err != nil {
		panic(err)
	}

	finishCh := make(chan struct{})
	defer close(finishCh)

	go func() {
		t := time.NewTicker(3 * time.Second)
		for i := 0; ; i++ {
			select {
			case <-t.C:
				broadcastBody, _ := ptypes.MarshalAny(&pb.MessageBody{
					JSON: fmt.Sprintf("{\"message\":\"hello from grpc(%d)!\"}", i),
				})

				broadcast, _ := ptypes.MarshalAny(&hpb.BroadcastMessageBody{
					Body: broadcastBody,
				})

				stream.Send(&hpb.Message{
					Kind: hpb.Message_Broadcast,
					Body: broadcast,
				})
			case <-finishCh:
				return
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Print(err)
			break
		}

		if msg.Kind == hpb.Message_OnBroadcastMessage {
			var body hpb.OnBroadcastMessageBody
			err := ptypes.UnmarshalAny(msg.Body, &body)
			if err != nil {
				log.Print(err)
				return
			}

			var bodyBody pb.MessageBody
			err = ptypes.UnmarshalAny(body.Body, &bodyBody)
			if err != nil {
				log.Print(err)
				return
			}

			log.Printf("%v: %v", body.From.Name, bodyBody.JSON)
		}

	}
}

func startWsServer(manager hibari.Manager) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(manager, websocket.ConnTransportOption{
			EncoderDecoder: websocketEncoderDecoder{},
		}, w, r)
	})

	http.ListenAndServe(":23032", nil)
}

func startGrpcServer(manager hibari.Manager) {
	l, err := net.Listen("tcp", ":3311")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	hpb.RegisterHibariServer(s, hgrpc.NewHibariServer(manager, hgrpc.HibariServerOption{
		TransOption: hgrpc.ConnTransportOption{
			EncoderDecoder: grpcEncoderDecoder{},
		},
	}))
	s.Serve(l)
}

func main() {
	manager := hibari.NewManager(roomSuggester{}, &hibari.ManagerOption{
		Authenticator: authenticator{
			mu: &sync.Mutex{},
		},
	})

	go startWsServer(manager)

	go func() {
		for {
			<-time.After(time.Second)
			startGrpcClient()
		}
	}()

	startGrpcServer(manager)
}
