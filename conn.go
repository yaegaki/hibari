package hibari

import (
	"context"
	"log"
	"sync"
	"time"
)

// Conn represents connection between server and client
type Conn interface {
	OnAuthenticationFailed()
	OnJoinFailed(error)

	OnJoin(r RoomInfo) error
	OnOtherUserJoin(u InRoomUser) error
	OnOtherUserLeave(u InRoomUser) error
	OnBroadcast(from InRoomUser, body interface{}) error
	Close()
}

type conn struct {
	manager   Manager
	trans     ConnTransport
	sendCh    chan Message
	closeCh   chan struct{}
	joinCh    chan struct{}
	closeOnce *sync.Once
}

// ConnTransport represents conn transport abstraction layer
type ConnTransport interface {
	Context() context.Context
	ReadMessage() (Message, error)
	WriteMessage(msg Message) error
	PingPeriod() time.Duration
	Ping() error
	Close()
}

// SendMessageError is occured if send message failed
type SendMessageError string

func (e SendMessageError) Error() string {
	return string(e)
}

// StartConn starts reads and writes pump
func StartConn(m Manager, ct ConnTransport) {
	c := &conn{
		manager:   m,
		trans:     ct,
		sendCh:    make(chan Message),
		closeCh:   make(chan struct{}),
		joinCh:    make(chan struct{}),
		closeOnce: &sync.Once{},
	}

	go c.readPump()
	go c.writePump()
}

func (c *conn) readPump() {
	defer c.Close()

	msg, err := c.trans.ReadMessage()
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}

	if msg.Kind != JoinMessage {
		log.Printf("Invalid message kind when join room: %v", msg.Kind)
		return
	}

	body, ok := msg.Body.(JoinMessageBody)
	if !ok {
		log.Printf("Invalid JoinMessageBody")
		return
	}

	userID := body.UserID
	roomID := body.RoomID

	room, err := c.manager.GetOrCreateRoom(roomID)
	if err != nil {
		log.Printf("Room not found: %v", roomID)
		return
	}

	err = room.Join(c.trans.Context(), userID, body.Secret, c)
	if err != nil {
		log.Printf("Join failed: %v", userID)
		return
	}

	// clear secret on memory
	body = JoinMessageBody{}

	// wait join
	select {
	case <-c.closeCh:
		return
	case <-c.joinCh:
	}

	defer room.Leave(userID)
	for {
		msg, err := c.trans.ReadMessage()
		if err != nil {
			return
		}

		switch msg.Kind {
		case BroadcastMessage:
			err = room.Broadcast(userID, msg.Body)
			if err != nil {
				log.Printf("Can not broadcast message: %v", err)
				return
			}
		case CustomMessage:
			customBody, ok := msg.Body.(CustomMessageBody)
			if !ok {
				log.Printf("Invalid CustomMessageBody")
				return
			}

			err = room.CustomMessage(userID, customBody.Kind, customBody.Body)
			if err != nil {
				log.Printf("Can not send custom message: %v", err)
				return
			}
		default:
			log.Printf("Forbidden message kind: %v", msg.Kind)
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
	defer c.trans.Close()
	var tickerC <-chan time.Time
	if c.trans.PingPeriod() > 0 {
		t := time.NewTicker(c.trans.PingPeriod())
		defer t.Stop()
		tickerC = t.C
	}

	for {
		select {
		case <-c.closeCh:
			return

		case msg := <-c.sendCh:
			if err := c.trans.WriteMessage(msg); err != nil {
				return
			}

		case <-tickerC:
			if err := c.trans.Ping(); err != nil {
				return
			}
		}
	}
}

func (c *conn) OnAuthenticationFailed() {
	c.safeSendMessage(Message{Kind: OnAuthenticationFailedMessage})
}

func (c *conn) OnJoinFailed(error) {
	c.safeSendMessage(Message{Kind: OnJoinFailedMessage})
}

func (c *conn) OnJoin(r RoomInfo) error {
	select {
	case <-c.closeCh:
		return AlreadyRoomClosedError{}
	case c.joinCh <- struct{}{}:
	}

	userMap := map[string]ShortUser{}
	for id, u := range r.UserMap {
		userMap[id] = u.ShortUser()
	}

	return c.safeSendMessageWithBody(OnJoinMessageBody{UserMap: userMap})
}

func (c *conn) OnOtherUserJoin(u InRoomUser) error {
	return c.safeSendMessageWithBody(OnOtherUserJoinMessageBody{User: u.ShortUser()})
}

func (c *conn) OnOtherUserLeave(u InRoomUser) error {
	return c.safeSendMessageWithBody(OnOtherUserLeaveMessageBody{User: u.ShortUser()})
}

func (c *conn) OnBroadcast(from InRoomUser, body interface{}) error {
	return c.safeSendMessageWithBody(OnBroadcastMessageBody{From: from.ShortUser(), Body: body})
}

func (c *conn) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
}

func (c *conn) safeSendMessageWithBody(body interface{}) error {
	msg, err := NewMessage(body)
	if err != nil {
		return err
	}
	return c.safeSendMessage(msg)
}

func (c *conn) safeSendMessage(msg Message) error {
	select {
	case <-c.closeCh:
		return SendMessageError("Send message failed")
	case c.sendCh <- msg:
		return nil
	default:
		return SendMessageError("Connection is stucked")
	}
}

// ShortUser converts InRoomUser to ShortUser
func (u InRoomUser) ShortUser() ShortUser {
	return ShortUser{
		Index: u.Index,
		Name:  u.User.Name,
	}
}
