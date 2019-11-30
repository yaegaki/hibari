package grpc

import (
	"context"
	"sync"

	"github.com/yaegaki/hibari"
	"github.com/yaegaki/hibari/grpc/pb"
)

type hibariServer struct {
	manager hibari.Manager
	option  HibariServerOption
}

// HibariServerOption configure hibariServer
type HibariServerOption struct {
	TransOption ConnTransportOption
}

// NewHibariServer creates instance of pb.HibariServer
func NewHibariServer(m hibari.Manager, option HibariServerOption) pb.HibariServer {
	return &hibariServer{
		manager: m,
		option:  option,
	}
}

func (s *hibariServer) Conn(stream pb.Hibari_ConnServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	c := &connTransport{
		ctx:       ctx,
		cancel:    cancel,
		stream:    stream,
		closeOnce: &sync.Once{},
		encDec:    s.option.TransOption.EncoderDecoder,
	}

	hibari.StartConn(s.manager, c, hibari.ConnOption{
		SendBufferSize: 10,
	})

	<-ctx.Done()
	return nil
}
