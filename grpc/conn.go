package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/yaegaki/hibari"
	"github.com/yaegaki/hibari/grpc/pb"
)

// ConnTransportOption configure ConnTransport
type ConnTransportOption struct {
	EncoderDecoder hibari.AnyMessageEncoderDecoder
}

type connTransport struct {
	closeCh   chan struct{}
	stream    pb.Hibari_ConnServer
	closeOnce *sync.Once
	encDec    hibari.AnyMessageEncoderDecoder
}

func (c *connTransport) Context() context.Context {
	return context.Background()
}

func (c *connTransport) ReadMessage() (hibari.Message, error) {
	msg, err := c.stream.Recv()
	if err != nil {
		return hibari.Message{}, err
	}

	kind := hibari.MessageKind(msg.Kind)

	result := hibari.Message{
		Kind: kind,
	}

	switch kind {
	case hibari.JoinMessage:
		var body pb.JoinMessageBody
		err = ptypes.UnmarshalAny(msg.Body, &body)
		result.Body = hibari.JoinMessageBody{
			UserID: body.UserID,
			Secret: body.Secret,
			RoomID: body.RoomID,
		}
	case hibari.BroadcastMessage:
		var body pb.BroadcastMessageBody
		err = ptypes.UnmarshalAny(msg.Body, &body)
		if err != nil {
			break
		}

		anyBody, err := c.encodeAnyMessageBody(body.Body)
		if err != nil {
			return hibari.Message{}, err
		}

		result.Body = anyBody
	case hibari.CustomMessage:
		var body pb.CustomMessageBody
		err = ptypes.UnmarshalAny(msg.Body, &body)
		if err != nil {
			break
		}

		anyBody, err := c.encodeAnyMessageBody(body.Body)
		if err != nil {
			return hibari.Message{}, err
		}

		result.Body = hibari.CustomMessageBody{
			Kind: hibari.CustomMessageKind(body.Kind),
			Body: anyBody,
		}
	default:
		err = fmt.Errorf("Invalid body")
	}

	if err != nil {
		return hibari.Message{}, err
	}

	return result, nil
}

func (c *connTransport) WriteMessage(msg hibari.Message) error {
	var body proto.Message
	valid := false
	switch msg.Kind {
	case hibari.OnAuthenticationFailedMessage:
		valid = true
	case hibari.OnJoinFailedMessage:
		valid = true
	case hibari.OnJoinMessage:
		temp, ok := msg.Body.(hibari.OnJoinMessageBody)
		if !ok {
			break
		}
		body = &pb.OnJoinMessageBody{
			UserMap: userMapFrom(temp.UserMap),
		}
	case hibari.OnOtherUserJoinMessage:
		temp, ok := msg.Body.(hibari.OnOtherUserJoinMessageBody)
		if !ok {
			break
		}
		body = &pb.OnOtherUserJoinMessageBody{
			User: shortUserFrom(temp.User),
		}
	case hibari.OnOtherUserLeaveMessage:
		temp, ok := msg.Body.(hibari.OnOtherUserLeaveMessageBody)
		if !ok {
			break
		}
		body = &pb.OnOtherUserLeaveMessageBody{
			User: shortUserFrom(temp.User),
		}
	case hibari.OnBroadcastMessage:
		temp, ok := msg.Body.(hibari.OnBroadcastMessageBody)
		if !ok {
			break
		}

		any, err := c.decodeAnyMessageBody(temp.Body)
		if err != nil {
			return err
		}

		body = &pb.OnBroadcastMessageBody{
			From: shortUserFrom(temp.From),
			Body: any,
		}
	}

	if !valid && body == nil {
		return fmt.Errorf("Invalid body")
	}

	pbBody, err := ptypes.MarshalAny(body)
	if err != nil {
		return err
	}

	result := &pb.Message{
		Kind: (pb.Message_MessageKind)(msg.Kind),
		Body: pbBody,
	}

	return c.stream.Send(result)
}

func (*connTransport) PingPeriod() time.Duration {
	return 0
}

func (*connTransport) Ping() error {
	return nil
}

func (c *connTransport) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
}

func (c *connTransport) encodeAnyMessageBody(body *any.Any) (interface{}, error) {
	if c.encDec != nil {
		result, err := c.encDec.EncodeAnyMessageBody(body)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return body, nil
}

func (c *connTransport) decodeAnyMessageBody(body interface{}) (*any.Any, error) {
	if c.encDec != nil {
		temp, err := c.encDec.DecodeAnyMessageBody(body)
		if err != nil {
			return nil, err
		}
		body = temp
	}

	any, ok := body.(*any.Any)
	if !ok {
		return nil, fmt.Errorf("Invalid any message body")
	}

	return any, nil
}

func userMapFrom(userMap map[string]hibari.ShortUser) map[string]*pb.ShortUser {
	m := map[string]*pb.ShortUser{}
	for k, v := range userMap {
		m[k] = shortUserFrom(v)
	}
	return m
}

func shortUserFrom(u hibari.ShortUser) *pb.ShortUser {
	return &pb.ShortUser{
		Index: (int32)(u.Index),
		Name:  u.Name,
	}
}
