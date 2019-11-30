package websocket

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v4"
	"github.com/yaegaki/hibari"
)

// ConnTransportOption configure ConnTransport
type ConnTransportOption struct {
	EncoderDecoder hibari.AnyMessageEncoderDecoder
}

type connTransport struct {
	ctx    context.Context
	cancel context.CancelFunc
	ws     *ws.Conn
	encDec hibari.AnyMessageEncoderDecoder
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 5000
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (c *connTransport) Context() context.Context {
	return context.Background()
}

func (c *connTransport) ReadMessage() (hibari.Message, error) {
	msgType, bin, err := c.ws.ReadMessage()
	if err != nil {
		return hibari.Message{}, err
	}

	if msgType != ws.BinaryMessage {
		return hibari.Message{}, fmt.Errorf("InvalidType message: %v", msgType)
	}

	var msg message
	err = msgpack.Unmarshal(bin, &msg)
	if err != nil {
		return hibari.Message{}, fmt.Errorf("UnmarshalError")
	}

	var body interface{}
	switch msg.Kind {
	case hibari.JoinMessage:
		temp, ok := msg.Body.(joinMessageBody)
		if !ok {
			break
		}
		body = hibari.JoinMessageBody{
			UserID: temp.UserID,
			Secret: temp.Secret,
			RoomID: temp.RoomID,
		}
	case hibari.BroadcastMessage:
		temp, ok := msg.Body.(broadcastMessageBody)
		if !ok {
			break
		}
		anyBody, err := c.encodeAnyMessageBody(temp)
		if err != nil {
			return hibari.Message{}, err
		}
		body = anyBody
	case hibari.CustomMessage:
		temp, ok := msg.Body.(customMessageBody)
		if !ok {
			break
		}

		anyBody, err := c.encodeAnyMessageBody(temp.Body)
		if err != nil {
			return hibari.Message{}, err
		}
		body = hibari.CustomMessageBody{
			Kind: temp.Kind,
			Body: anyBody,
		}
	}

	if body == nil {
		return hibari.Message{}, fmt.Errorf("Invalid body")
	}

	return hibari.Message{
		Kind: msg.Kind,
		Body: body,
	}, nil
}

func (c *connTransport) WriteMessage(msg hibari.Message) error {
	var body interface{}
	switch msg.Kind {
	case hibari.OnAuthenticationFailedMessage:
		body = struct{}{}
	case hibari.OnJoinFailedMessage:
		body = struct{}{}
	case hibari.OnJoinMessage:
		temp, ok := msg.Body.(hibari.OnJoinMessageBody)
		if !ok {
			break
		}
		body = onJoinMessageBody{
			UserMap: userMapFrom(temp.UserMap),
		}
	case hibari.OnOtherUserJoinMessage:
		temp, ok := msg.Body.(hibari.OnOtherUserJoinMessageBody)
		if !ok {
			break
		}
		body = onOtherUserJoinMessageBody{
			User: shortUserFrom(temp.User),
		}
	case hibari.OnOtherUserLeaveMessage:
		temp, ok := msg.Body.(hibari.OnOtherUserLeaveMessageBody)
		if !ok {
			break
		}
		body = onOtherUserLeaveMessageBody{
			User: shortUserFrom(temp.User),
		}
	case hibari.OnBroadcastMessage:
		temp, ok := msg.Body.(hibari.OnBroadcastMessageBody)
		if !ok {
			break
		}

		bytes, err := c.decodeAnyMessageBody(temp.Body)
		if err != nil {
			return err
		}

		body = onBroadcastMessageBody{
			From: shortUserFrom(temp.From),
			Body: bytes,
		}
	default:
	}

	if body == nil {
		return fmt.Errorf("Invalid body")
	}

	bin, err := msgpack.Marshal(message{
		Kind: msg.Kind,
		Body: body,
	})
	if err != nil {
		return err
	}

	return c.ws.WriteMessage(ws.BinaryMessage, bin)
}

func (*connTransport) PingPeriod() time.Duration {
	return pingPeriod
}

func (c *connTransport) Ping() error {
	return c.ws.WriteMessage(ws.PingMessage, []byte{})
}

func (c *connTransport) Close() {
	c.ws.WriteMessage(ws.CloseMessage, []byte{})
	c.ws.Close()
	c.cancel()
}

func (c *connTransport) encodeAnyMessageBody(body []byte) (interface{}, error) {
	if c.encDec != nil {
		result, err := c.encDec.EncodeAnyMessageBody(body)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return body, nil
}

func (c *connTransport) decodeAnyMessageBody(body interface{}) ([]byte, error) {
	if c.encDec != nil {
		temp, err := c.encDec.DecodeAnyMessageBody(body)
		if err != nil {
			return nil, err
		}
		body = temp
	}

	bin, ok := body.([]byte)
	if !ok {
		return nil, fmt.Errorf("Invalid any message body")
	}

	return bin, nil
}

// ServeWs starts serve websocket
func ServeWs(m hibari.Manager, o ConnTransportOption, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &connTransport{
		ctx:    ctx,
		cancel: cancel,
		ws:     ws,
		encDec: o.EncoderDecoder,
	}

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	hibari.StartConn(m, c, hibari.ConnOption{SendBufferSize: 10})
}
