package hibari

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Room .
type Room interface {
	Run()
	Shutdown() AsyncOperation
	Enqueue(f func()) AsyncOperation

	ID() string
	RoomInfo() RoomInfo
	GetConn(id string) (Conn, error)
	Join(ctx context.Context, user User, secret string, conn Conn) error
	Leave(id string) error
	Broadcast(id string, body interface{}) error
	CustomMessage(id string, kind CustomMessageKind, body interface{}) error
	Closed() bool
}

// RoomOption configures room
type RoomOption struct {
	Deadline time.Duration
	Logger   Logger
}

type room struct {
	id           string
	option       RoomOption
	manager      Manager
	handler      RoomHandler
	userMap      map[string]*roomUser
	msgCh        chan internalMessage
	shutdownCh   chan struct{}
	currentIndex int
	logger       Logger
	closed       bool
	runOnce      *sync.Once
	shutdownOnce *sync.Once
}

// RoomInfo is room information
type RoomInfo struct {
	ID      string
	UserMap map[string]InRoomUser
}

// AlreadyRoomClosedError is occurred if room was already closed
type AlreadyRoomClosedError struct{}

func (AlreadyRoomClosedError) Error() string {
	return "Already room was closed"
}

// AlreadyUserJoinedError is occurred if user was already exists in the room
type AlreadyUserJoinedError struct {
	User User
}

func (e AlreadyUserJoinedError) Error() string {
	return fmt.Sprintf("Already user was joined: ID(%v) Name(%v)", e.User.ID, e.User.Name)
}

type roomUser struct {
	ctx   context.Context
	u     User
	index int
	conn  Conn
}

func (u roomUser) InRoomUser() InRoomUser {
	return InRoomUser{
		Index: u.index,
		User:  u.u,
	}
}

// RoomAllocator creates new room
type RoomAllocator interface {
	Alloc(ctx context.Context, id string, m Manager) (Room, error)
	Free(id string)
}

type internalRoomAllocator struct{}

func (internalRoomAllocator) Alloc(_ context.Context, id string, m Manager) (Room, error) {
	return NewRoom(id, m, nil, RoomOption{}), nil
}

func (internalRoomAllocator) Free(string) {
}

// RoomHandler customizes room behavior
type RoomHandler interface {
	ValidateJoinUser(ctx context.Context, r Room, u User) error

	OnJoinUser(r Room, user InRoomUser)
	OnDisconnectUser(r Room, user InRoomUser)
	OnCustomMessage(r Room, user InRoomUser, kind CustomMessageKind, body interface{})
	OnShutdown()
}

type internalRoomHandler struct{}

func (internalRoomHandler) ValidateJoinUser(context.Context, Room, User) error {
	return nil
}

func (internalRoomHandler) OnJoinUser(Room, InRoomUser) {
}

func (internalRoomHandler) OnDisconnectUser(Room, InRoomUser) {
}

func (internalRoomHandler) OnCustomMessage(Room, InRoomUser, CustomMessageKind, interface{}) {
}

func (internalRoomHandler) OnShutdown() {
}

// NewRoom creates a new Room
func NewRoom(roomID string, m Manager, rh RoomHandler, option RoomOption) Room {
	if rh == nil {
		rh = internalRoomHandler{}
	}

	if option.Deadline <= 0 {
		option.Deadline = 10 * time.Second
	}

	if option.Logger == nil {
		option.Logger = stdLogger{}
	}

	r := &room{
		id:           roomID,
		option:       option,
		manager:      m,
		handler:      rh,
		userMap:      map[string]*roomUser{},
		msgCh:        make(chan internalMessage),
		shutdownCh:   make(chan struct{}),
		logger:       option.Logger,
		runOnce:      &sync.Once{},
		shutdownOnce: &sync.Once{},
	}

	return r
}

func (r *room) Run() {
	r.runOnce.Do(func() {
		t := time.NewTicker(r.option.Deadline)
		defer t.Stop()

		updatedTime := time.Now()
		for {
			select {
			case msg := <-r.msgCh:
				// r.logger.Printf("%v", msg)
				if msg.kind == internalShutdownMessage {
					r.handler.OnShutdown()
					break
				}

				r.handleMessage(msg)
				updatedTime = time.Now()
			case <-t.C:
				if len(r.userMap) > 0 {
					updatedTime = time.Now()
					continue
				}

				d := time.Now().Sub(updatedTime)
				if d >= r.option.Deadline {
					t.Stop()
					r.Shutdown()
				}
			}
		}
	})
}

func (r *room) Shutdown() AsyncOperation {
	finishCh := make(chan struct{})

	go func() {
		defer close(finishCh)

		r.shutdownOnce.Do(func() {
			close(r.shutdownCh)

			msg := internalMessage{
				kind: internalShutdownMessage,
			}
			r.msgCh <- msg

			for _, u := range r.userMap {
				u.conn.Close()
			}

			r.closed = true
			r.manager.NotifyRoomClosed(r.id)
		})
	}()

	return NewAsyncOperation(finishCh)
}

func (r *room) Enqueue(f func()) AsyncOperation {
	finishCh := make(chan struct{})

	go func() {
		err := r.safeSendMessage(internalMessage{
			kind: internalInvokeMessage,
			body: internalInvokeMessageBody{
				f: func() {
					defer close(finishCh)
					f()
				},
			},
		})

		if err != nil {
			close(finishCh)
		}
	}()

	return NewAsyncOperation(finishCh)
}

func (r *room) ID() string {
	return r.id
}

func (r *room) RoomInfo() RoomInfo {
	userMap := map[string]InRoomUser{}
	for id, u := range r.userMap {
		userMap[id] = u.InRoomUser()
	}

	return RoomInfo{
		ID:      r.id,
		UserMap: userMap,
	}
}

func (r *room) GetConn(id string) (Conn, error) {
	u, ok := r.userMap[id]
	if !ok {
		return nil, fmt.Errorf("Not found")
	}

	return u.conn, nil
}

func (r *room) Join(ctx context.Context, user User, secret string, conn Conn) error {
	msg := internalMessage{
		kind: internalJoinMessage,
		body: internalJoinMessageBody{
			ctx:  ctx,
			user: user,
			conn: conn,
		},
	}
	return r.safeSendMessage(msg)
}

func (r *room) Leave(id string) error {
	msg := internalMessage{
		kind: internalPreLeaveMessage,
		body: internalPreLeaveMessageBody{
			userID: id,
		},
	}
	return r.safeSendMessage(msg)
}

func (r *room) Broadcast(id string, body interface{}) error {
	msg := internalMessage{
		kind: internalBroadcastMessage,
		body: internalBroadcastMessageBody{
			userID: id,
			body:   body,
		},
	}
	return r.safeSendMessage(msg)
}

func (r *room) CustomMessage(id string, kind CustomMessageKind, body interface{}) error {
	msg := internalMessage{
		kind: internalCustomMessage,
		body: internalCustomMessageBody{
			userID: id,
			kind:   kind,
			body:   body,
		},
	}
	return r.safeSendMessage(msg)
}

func (r *room) Closed() bool {
	return r.closed
}

func (r *room) safeSendMessage(msg internalMessage) error {
	select {
	case <-r.shutdownCh:
		return AlreadyRoomClosedError{}
	case r.msgCh <- msg:
		return nil
	}
}

func (r *room) handleMessage(msg internalMessage) {
	switch msg.kind {
	case internalJoinMessage:
		if body, ok := msg.body.(internalJoinMessageBody); ok {
			r.handleJoinMessage(body)
		}
	case internalPreLeaveMessage:
		if body, ok := msg.body.(internalPreLeaveMessageBody); ok {
			r.handlePreLeaveMessage(body)
		}
	case internalLeaveMessage:
		if body, ok := msg.body.(internalLeaveMessageBody); ok {
			r.handleLeaveMessage(body)
		}
	case internalBroadcastMessage:
		if body, ok := msg.body.(internalBroadcastMessageBody); ok {
			r.handleBroadcastMessage(body)
		}
	case internalCustomMessage:
		if body, ok := msg.body.(internalCustomMessageBody); ok {
			r.handleCustomMessage(body)
		}
	case internalInvokeMessage:
		if body, ok := msg.body.(internalInvokeMessageBody); ok {
			body.f()
		}
	default:
		break
	}
}

func (r *room) handleJoinMessage(body internalJoinMessageBody) {
	if _, ok := r.userMap[body.user.ID]; ok {
		body.conn.OnJoinFailed(AlreadyUserJoinedError{
			User: body.user,
		})
		body.conn.Close()
		return
	}

	joinedUser := &roomUser{
		ctx:   body.ctx,
		u:     body.user,
		index: r.currentIndex,
		conn:  body.conn,
	}

	err := r.handler.ValidateJoinUser(joinedUser.ctx, r, joinedUser.u)
	if err != nil {
		joinedUser.conn.OnJoinFailed(err)
		joinedUser.conn.Close()
		return
	}

	joinedRoomUser := joinedUser.InRoomUser()
	for _, u := range r.userMap {
		err = u.conn.OnOtherUserJoin(joinedRoomUser)
		if err != nil {
			r.disconnect(*u)
		}
	}

	r.currentIndex++
	r.userMap[joinedUser.u.ID] = joinedUser

	r.handler.OnJoinUser(r, joinedRoomUser)

	err = joinedUser.conn.OnJoin(r.RoomInfo())
	if err != nil {
		joinedUser.conn.Close()
		go r.Leave(joinedUser.u.ID)
		return
	}
}

func (r *room) handlePreLeaveMessage(body internalPreLeaveMessageBody) {
	u, ok := r.userMap[body.userID]
	if !ok {
		return
	}

	r.disconnect(*u)
}

func (r *room) handleLeaveMessage(body internalLeaveMessageBody) {
	if _, ok := r.userMap[body.user.u.ID]; ok {
		delete(r.userMap, body.user.u.ID)
	}

	inRoomUser := body.user.InRoomUser()
	for _, u := range r.userMap {
		err := u.conn.OnOtherUserLeave(inRoomUser)
		if err != nil {
			r.disconnect(*u)
		}
	}

	r.handler.OnDisconnectUser(r, body.user.InRoomUser())
}

func (r *room) handleBroadcastMessage(body internalBroadcastMessageBody) {
	u, ok := r.userMap[body.userID]
	if !ok {
		return
	}

	inRoomUser := u.InRoomUser()
	for _, u := range r.userMap {
		err := u.conn.OnBroadcast(inRoomUser, body.body)
		if err != nil {
			r.disconnect(*u)
		}
	}
}

func (r *room) handleCustomMessage(body internalCustomMessageBody) {
	u, ok := r.userMap[body.userID]
	if !ok {
		return
	}

	r.handler.OnCustomMessage(r, u.InRoomUser(), body.kind, body.body)
}

func (r *room) disconnect(u roomUser) {
	delete(r.userMap, u.u.ID)
	u.conn.Close()
	go func() {
		msg := internalMessage{
			kind: internalLeaveMessage,
			body: internalLeaveMessageBody{
				user: u,
			},
		}

		r.safeSendMessage(msg)
	}()
}
