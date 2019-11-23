package hibari

import "sync"

import "context"

// Room .
type Room interface {
	Run()
	Shutdown() chan struct{}

	ID() string
	RoomInfo() RoomInfo
	Join(userCtx context.Context, id, secret string, conn Conn) error
	Leave(id string) error
	Broadcast(id string, body interface{}) error
	Closed() bool
}

type room struct {
	id           string
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
	UserMap map[string]InRoomUser
}

// AlreadyRoomClosedError is occurred if room was already closed
type AlreadyRoomClosedError struct{}

func (AlreadyRoomClosedError) Error() string {
	return "Already room was closed"
}

type roomUser struct {
	u       User
	index   int
	conn    Conn
	userCtx context.Context
}

func (u roomUser) InRoomUser() InRoomUser {
	return InRoomUser{
		Index: u.index,
		Name:  u.u.Name,
	}
}

// RoomAllocator creates new room
type RoomAllocator interface {
	Alloc(id string, m Manager) (Room, error)
}

type internalRoomAllocator struct{}

func (internalRoomAllocator) Alloc(id string, m Manager) (Room, error) {
	return NewRoom(id, m, nil, nil), nil
}

// RoomHandler customizes room behavior
type RoomHandler interface {
	Authenticate(userCtx context.Context, r Room, id, secret string) (User, error)
	ValidateJoinUser(userCtx context.Context, r Room, u User) error

	OnDisconnectUser(r Room, id string)
}

type internalRoomHandler struct{}

func (internalRoomHandler) Authenticate(_ context.Context, _ Room, id, secret string) (User, error) {
	return User{
		ID:   id,
		Name: "",
	}, nil
}

func (internalRoomHandler) ValidateJoinUser(context.Context, Room, User) error {
	return nil
}

func (internalRoomHandler) OnDisconnectUser(Room, string) {
}

// NewRoom creates a new Room
func NewRoom(roomID string, m Manager, rh RoomHandler, logger Logger) Room {
	if rh == nil {
		rh = internalRoomHandler{}
	}
	if logger == nil {
		logger = stdLogger{}
	}
	r := &room{
		id:           roomID,
		manager:      m,
		handler:      rh,
		userMap:      map[string]*roomUser{},
		msgCh:        make(chan internalMessage),
		shutdownCh:   make(chan struct{}),
		logger:       logger,
		runOnce:      &sync.Once{},
		shutdownOnce: &sync.Once{},
	}

	return r
}

func (r *room) Run() {
	r.runOnce.Do(func() {
		for {
			msg := <-r.msgCh
			// r.logger.Printf("%v", msg)
			if msg.kind == internalShutdownMessage {
				break
			}

			r.handleMessage(msg)
		}
	})
}

func (r *room) Shutdown() chan struct{} {
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

	return finishCh
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
		UserMap: userMap,
	}
}

func (r *room) Join(userCtx context.Context, id, secret string, conn Conn) error {
	u, err := r.handler.Authenticate(userCtx, r, id, secret)
	if err != nil {
		conn.OnAuthenticationFailed()
		conn.Close()
		return err
	}

	msg := internalMessage{
		kind: internalJoinMessage,
		body: internalJoinMessageBody{
			user:    u,
			conn:    conn,
			userCtx: userCtx,
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
		break
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
	default:
		break
	}
}

func (r *room) handleJoinMessage(body internalJoinMessageBody) {
	if _, ok := r.userMap[body.user.ID]; ok {
		r.logger.Printf("Already joined: %v")
		body.conn.Close()
		return
	}

	joinedUser := &roomUser{
		u:       body.user,
		index:   r.currentIndex,
		conn:    body.conn,
		userCtx: body.userCtx,
	}

	err := r.handler.ValidateJoinUser(joinedUser.userCtx, r, joinedUser.u)
	if err != nil {
		joinedUser.conn.OnJoinFailed(err)
		joinedUser.conn.Close()
		return
	}

	err = joinedUser.conn.OnJoin(r.RoomInfo())
	if err != nil {
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

	r.handler.OnDisconnectUser(r, body.user.u.ID)
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
