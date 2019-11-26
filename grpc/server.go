package grpc

import (
	"sync"

	"github.com/yaegaki/hibari"
	"github.com/yaegaki/hibari/grpc/pb"
)

type hibariServer struct {
	manager hibari.Manager
}

// NewHibariServer creates instance of pb.HibariServer
func NewHibariServer(m hibari.Manager) pb.HibariServer {
	return &hibariServer{
		manager: m,
	}
}

func (s *hibariServer) Conn(stream pb.Hibari_ConnServer) error {
	closeCh := make(chan struct{})
	c := &connTransport{
		closeCh:   closeCh,
		stream:    stream,
		closeOnce: &sync.Once{},
	}

	hibari.StartConn(s.manager, c, hibari.ConnOption{
		SendBufferSize: 10,
	})

	<-closeCh
	return nil
}
