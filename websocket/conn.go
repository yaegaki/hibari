package websocket

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v4"
	"github.com/yaegaki/hibari"
)

type conn struct {
	ws        *ws.Conn
	manager   hibari.Manager
	sendCh    chan []byte
	closeCh   chan struct{}
	joinCh    chan struct{}
	closeOnce *sync.Once
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

func (c *conn) readPump() {
	defer func() {
		c.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	msg, err := c.readWsMessage()
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}

	if msg.Kind != joinMessage {
		log.Printf("Invalid message kind: %v", msg.Kind)
		return
	}

	var body joinMessageBody
	err = msgpack.Unmarshal(msg.Body, &body)
	if err != nil {
		log.Printf("Unmarshal error: %v", err)
		return
	}
	userID := body.UserID

	room, err := c.manager.GetOrCreateRoom(body.RoomID)
	if err != nil {
		log.Printf("Room not found: %v", body.RoomID)
		return
	}

	ctx := context.Background()
	err = room.Join(ctx, userID, body.Secret, c)
	if err != nil {
		log.Printf("Join failed: %v", userID)
		return
	}

	select {
	case <-c.closeCh:
		return
	case <-c.joinCh:
	}

	defer room.Leave(userID)

	for {
		msg, err := c.readWsMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		if msg.Kind != broadcastMessage {
			log.Printf("Forbidden message kind: %v", msg.Kind)
			return
		}

		err = room.Broadcast(userID, msg.Body)
		if err != nil {
			log.Printf("Can not broadcast message: %v", err)
			return
		}

		select {
		case <-c.closeCh:
			return
		default:
		}
	}
}

func (c *conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		c.ws.Close()
	}()

	for {
		select {
		case <-c.closeCh:
			c.ws.WriteMessage(ws.CloseMessage, []byte{})
			return

		case bin := <-c.sendCh:
			if err := c.ws.WriteMessage(ws.BinaryMessage, bin); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.ws.WriteMessage(ws.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *conn) OnAuthenticationFailed() {
	c.safeSendMessage(onAuthenticationFailedMessage, nil)
}

func (c *conn) OnJoinFailed(error) {
	c.safeSendMessage(onJoinFailedMessage, nil)
}

func (c *conn) OnJoin(r hibari.RoomInfo) error {
	select {
	case <-c.closeCh:
		return fmt.Errorf("Already closed conn")
	case c.joinCh <- struct{}{}:
	}

	userMap := map[string]inRoomUser{}
	for id, u := range r.UserMap {
		userMap[id] = toInRoomUser(u)
	}

	msg := onJoinMessageBody{
		UserMap: userMap,
	}
	return c.safeSendMessage(onJoinMessage, msg)
}

func (c *conn) OnOtherUserJoin(u hibari.InRoomUser) error {
	msg := otherUserJoinMessageBody{User: toInRoomUser(u)}
	return c.safeSendMessage(onOtherUserJoinMessage, msg)
}

func (c *conn) OnOtherUserLeave(u hibari.InRoomUser) error {
	msg := otherUserLeaveMessageBody{User: toInRoomUser(u)}
	return c.safeSendMessage(onOtherUserLeaveMessage, msg)
}

func (c *conn) OnBroadcast(from hibari.InRoomUser, body interface{}) error {
	msg := onBroadcastMessageBody{From: toInRoomUser(from), Body: body}
	return c.safeSendMessage(onBroadcastMessage, msg)
}

func (c *conn) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
}

type canNotSendMessageError int

func (canNotSendMessageError) Error() string {
	return "Can not send message"
}

func toInRoomUser(u hibari.InRoomUser) inRoomUser {
	return inRoomUser{
		Index: u.Index,
		Name:  u.Name,
	}
}

func (c *conn) readWsMessage() (clientToServerMessage, error) {
	msgType, bin, err := c.ws.ReadMessage()
	if err != nil {
		return clientToServerMessage{}, fmt.Errorf("Can not read message")
	}

	if msgType != ws.BinaryMessage {
		return clientToServerMessage{}, nil
	}

	var msg clientToServerMessage
	err = msgpack.Unmarshal(bin, &msg)
	if err != nil {
		return clientToServerMessage{}, err
	}

	return msg, err
}

func (c *conn) safeSendMessage(kind messageKind, body interface{}) error {
	bin, err := msgpack.Marshal(newMessage(kind, body))
	if err != nil {
		return err
	}

	select {
	case <-c.closeCh:
		return canNotSendMessageError(0)
	case c.sendCh <- bin:
		return nil
	}
}

// ServeWs starts serve websocket
func ServeWs(m hibari.Manager, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn := conn{
		ws:        ws,
		manager:   m,
		sendCh:    make(chan []byte),
		closeCh:   make(chan struct{}),
		joinCh:    make(chan struct{}),
		closeOnce: &sync.Once{},
	}

	go conn.readPump()
	go conn.writePump()
}
