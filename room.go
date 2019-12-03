package hibari

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Room .
type Room interface {
	Run()
	Shutdown() AsyncOperation
	Enqueue(f func()) (AsyncOperation, error)

	// ForGoroutine creates goroutine safe a room.
	ForGoroutine() Room

	ID() string
	RoomInfo() (RoomInfo, error)
	ExistsUser(id string) bool
	GetConn(id string) (Conn, error)
	Join(ctx context.Context, user User, conn Conn) error
	JoinWithoutInterception(ctx context.Context, user User, conn Conn) error
	Leave(id string) error
	Broadcast(id string, body interface{}) error
	BroadcastWithoutInterception(id string, body interface{}) error
	CustomMessage(id string, kind CustomMessageKind, body interface{}) error
	CustomMessageWithoutInterception(id string, kind CustomMessageKind, body interface{}) error
	Closed() bool
}

// RoomOption configures room
type RoomOption struct {
	Deadline    time.Duration
	Interceptor RoomInterceptor
	Logger      Logger
}

type room struct {
	id           string
	option       RoomOption
	manager      Manager
	handler      RoomHandler
	interceptor  RoomInterceptor
	userMap      map[string]*roomUser
	msgCh        chan internalMessage
	shutdownCh   chan struct{}
	currentIndex int
	logger       Logger
	closed       bool
	runOnce      *sync.Once
	shutdownOnce *sync.Once
}

type goroutineSafeRoom struct {
	r *room
}

// RoomInfo is room information
type RoomInfo struct {
	ID      string
	UserMap map[string]InRoomUser
}

// ErrAlreadyRoomClosed is returned when access to the room when that was already closed.
var ErrAlreadyRoomClosed = errors.New("already room was closed")

// ErrJoinRoomPending is returned when the join request is pending.
var ErrJoinRoomPending = errors.New("the join request is pending")

// ErrJoinRoomDenied is returend when the join request was denied.
var ErrJoinRoomDenied = errors.New("the join request was denied")

// ErrJoinRoomConnClosed is returned when the user's conn was already closed.
var ErrJoinRoomConnClosed = errors.New("the join request was denied")

// AlreadyUserJoinedError represents a user is already joined room.
type AlreadyUserJoinedError struct {
	User User
}

func (e AlreadyUserJoinedError) Error() string {
	return fmt.Sprintf("the user was already joined: ID(%v) Name(%v)", e.User.ID, e.User.Name)
}

type roomUser struct {
	ctx   context.Context
	u     User
	index int
	conn  Conn
}

func (u roomUser) InRoomUser() InRoomUser {
	return InRoomUser{
		Ctx:   u.ctx,
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

	if option.Interceptor == nil {
		option.Interceptor = NilRoomInterceptor{}
	}

	if option.Logger == nil {
		option.Logger = stdLogger{}
	}

	r := &room{
		id:           roomID,
		option:       option,
		manager:      m,
		handler:      rh,
		interceptor:  option.Interceptor,
		userMap:      map[string]*roomUser{},
		msgCh:        make(chan internalMessage),
		shutdownCh:   make(chan struct{}),
		logger:       option.Logger,
		runOnce:      &sync.Once{},
		shutdownOnce: &sync.Once{},
	}

	return r.ForGoroutine()
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

// unsafeEnqueue must be invoked on goroutine for avoid deadlock.
func (r *room) unsafeEnqueue(finishCh chan struct{}, f func()) error {
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
		return err
	}

	return nil
}

func (r *room) Enqueue(f func()) (AsyncOperation, error) {
	finishCh := make(chan struct{})
	var err error
	go func() {
		err = r.unsafeEnqueue(finishCh, f)
	}()

	if err != nil {
		return nil, err
	}

	return NewAsyncOperation(finishCh), nil
}

func (r *room) ForGoroutine() Room {
	return &goroutineSafeRoom{
		r: r,
	}
}

func (r *room) ID() string {
	return r.id
}

func (r *room) RoomInfo() (RoomInfo, error) {
	userMap := map[string]InRoomUser{}
	for id, u := range r.userMap {
		userMap[id] = u.InRoomUser()
	}

	return RoomInfo{
		ID:      r.id,
		UserMap: userMap,
	}, nil
}

func (r *room) ExistsUser(id string) bool {
	_, ok := r.userMap[id]
	return ok
}

func (r *room) GetConn(id string) (Conn, error) {
	u, ok := r.userMap[id]
	if !ok {
		return nil, UserNotFoundError{ID: id}
	}

	return u.conn, nil
}

// AlreadyInRoomError is occurred if user is already joined the room.
type AlreadyInRoomError struct {
	User User
}

func (e AlreadyInRoomError) Error() string {
	return fmt.Sprintf("User join failed because already in the room: %v(%v)", e.User.Name, e.User.ID)
}

func (r *room) Join(ctx context.Context, user User, conn Conn) error {
	_, err := r.join(ctx, user, conn, true)
	return err
}

func (r *room) JoinWithoutInterception(ctx context.Context, user User, conn Conn) error {
	_, err := r.join(ctx, user, conn, false)
	return err
}

func (r *room) join(ctx context.Context, user User, conn Conn, interception bool) (context.Context, error) {
	if _, ok := r.userMap[user.ID]; ok {
		return nil, AlreadyInRoomError{User: user}
	}

	joinedUser := &roomUser{
		ctx:   ctx,
		u:     user,
		index: r.currentIndex,
		conn:  conn,
	}

	err := r.handler.ValidateJoinUser(joinedUser.ctx, r, joinedUser.u)
	if err != nil {
		return nil, err
	}

	if interception {
		result, pendingCtx := r.interceptor.InterceptJoin(joinedUser.ctx, r, joinedUser.u, joinedUser.conn)
		switch result {
		case JoinInterceptionPending:
			return pendingCtx, ErrJoinRoomPending
		case JoinInterceptionDeny:
			return nil, ErrJoinRoomDenied
		default:
		}
	}

	r.userMap[joinedUser.u.ID] = joinedUser
	r.currentIndex++

	// r.RoomInfo() success always.
	roomInfo, _ := r.RoomInfo()
	err = joinedUser.conn.OnJoin(roomInfo)

	if err != nil {
		// already user is joined but disconnected. so room must send leave message for other users.
		r.enqueueLeave(joinedUser.u.ID)
		return nil, err
	}

	// user join completed.

	joinedRoomUser := joinedUser.InRoomUser()
	for _, u := range r.userMap {
		if u.index == joinedRoomUser.Index {
			continue
		}

		err = u.conn.OnOtherUserJoin(joinedRoomUser)
		if err != nil {
			r.enqueueLeave(u.u.ID)
		}
	}

	// Notify joinUser after all message sent
	r.handler.OnJoinUser(r, joinedRoomUser)

	return nil, nil
}

// UserNotFoundError is occurred if user not found.
type UserNotFoundError struct {
	ID string
}

func (e UserNotFoundError) Error() string {
	return fmt.Sprintf("user not found: %v", e.ID)
}

func (r *room) Leave(id string) error {
	user, ok := r.userMap[id]
	if !ok {
		return UserNotFoundError{ID: id}
	}

	delete(r.userMap, id)
	user.conn.Close()

	inRoomUser := user.InRoomUser()
	for _, u := range r.userMap {
		err := u.conn.OnOtherUserLeave(inRoomUser)
		if err != nil {
			r.enqueueLeave(u.u.ID)
		}
	}

	r.handler.OnDisconnectUser(r, inRoomUser)
	return nil
}

func (r *room) Broadcast(id string, body interface{}) error {
	return r.broadcast(id, body, true)
}

func (r *room) BroadcastWithoutInterception(id string, body interface{}) error {
	return r.broadcast(id, body, false)
}

func (r *room) broadcast(id string, body interface{}, interception bool) error {
	user, ok := r.userMap[id]
	if !ok {
		return UserNotFoundError{ID: id}
	}

	inRoomUser := user.InRoomUser()
	if interception {
		result := r.interceptor.InterceptBroadcast(r, inRoomUser, body)
		switch result {
		case MessageInterceptionDeny:
			return nil
		case MessageInterceptionDisconnect:
			r.Leave(id)
			return nil
		default:
		}
	}

	for _, u := range r.userMap {
		err := u.conn.OnBroadcast(inRoomUser, body)
		if err != nil {
			r.enqueueLeave(u.u.ID)
		}
	}

	return nil
}

func (r *room) CustomMessage(id string, kind CustomMessageKind, body interface{}) error {
	return r.customMessage(id, kind, body, true)
}

func (r *room) CustomMessageWithoutInterception(id string, kind CustomMessageKind, body interface{}) error {
	return r.customMessage(id, kind, body, false)
}

func (r *room) customMessage(id string, kind CustomMessageKind, body interface{}, interception bool) error {
	user, ok := r.userMap[id]
	if !ok {
		return UserNotFoundError{ID: id}
	}

	inRoomUser := user.InRoomUser()
	if interception {
		result := r.interceptor.InterceptCustomMessage(r, inRoomUser, kind, body)
		switch result {
		case MessageInterceptionDeny:
			return nil
		case MessageInterceptionDisconnect:
			r.Leave(id)
			return nil
		default:
		}
	}

	r.handler.OnCustomMessage(r, inRoomUser, kind, body)
	return nil
}

func (r *room) Closed() bool {
	return r.closed
}

func (r *room) safeSendMessage(msg internalMessage) error {
	select {
	case <-r.shutdownCh:
		return ErrAlreadyRoomClosed
	case r.msgCh <- msg:
		return nil
	}
}

func (r *room) handleMessage(msg internalMessage) {
	switch msg.kind {
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

func (r *room) handleBroadcastMessage(body internalBroadcastMessageBody) {
	r.Broadcast(body.userID, body.body)
}

func (r *room) handleCustomMessage(body internalCustomMessageBody) {
	r.CustomMessage(body.userID, body.kind, body.body)
}

func (r *room) enqueueLeave(id string) {
	r.Enqueue(func() {
		r.Leave(id)
	})
}

func (r *goroutineSafeRoom) Run() {
	r.r.Run()
}

func (r *goroutineSafeRoom) Shutdown() AsyncOperation {
	return r.r.Shutdown()
}

func (r *goroutineSafeRoom) Enqueue(f func()) (AsyncOperation, error) {
	finishCh := make(chan struct{})
	err := r.r.unsafeEnqueue(finishCh, f)

	if err != nil {
		return nil, err
	}

	return NewAsyncOperation(finishCh), nil
}

// ForGoroutine creates goroutine safe room.
func (r *goroutineSafeRoom) ForGoroutine() Room {
	return r
}

func (r *goroutineSafeRoom) ID() string {
	return r.r.ID()
}

func (r *goroutineSafeRoom) RoomInfo() (roomInfo RoomInfo, innerErr error) {
	op, err := r.Enqueue(func() {
		roomInfo, innerErr = r.r.RoomInfo()
	})

	if err != nil {
		return RoomInfo{}, err
	}

	<-op.Done()
	return
}

func (r *goroutineSafeRoom) ExistsUser(id string) bool {
	var exists bool
	op, err := r.Enqueue(func() {
		exists = r.r.ExistsUser(id)
	})

	if err != nil {
		return false
	}

	<-op.Done()
	return exists
}

func (r *goroutineSafeRoom) GetConn(id string) (conn Conn, innerErr error) {
	op, err := r.Enqueue(func() {
		conn, innerErr = r.r.GetConn(id)
	})

	if err != nil {
		return nil, err
	}

	<-op.Done()
	return
}

func (r *goroutineSafeRoom) Join(ctx context.Context, user User, conn Conn) (innerErr error) {
	return r.join(ctx, user, conn, true)
}

func (r *goroutineSafeRoom) JoinWithoutInterception(ctx context.Context, user User, conn Conn) (innerErr error) {
	return r.join(ctx, user, conn, false)
}

func (r *goroutineSafeRoom) join(ctx context.Context, user User, conn Conn, interception bool) (innerErr error) {
	var pendingCtx context.Context
	op, err := r.Enqueue(func() {
		pendingCtx, innerErr = r.r.join(ctx, user, conn, interception)
	})

	if err != nil {
		return err
	}

	<-op.Done()
	if innerErr != ErrJoinRoomPending {
		return innerErr
	}

	// join requesit is pending.

	resultCh := make(chan error)
	go func() {
		select {
		case <-pendingCtx.Done():
			if r.ExistsUser(user.ID) {
				resultCh <- nil
				return
			}

			resultCh <- ErrJoinRoomDenied
		case <-ctx.Done():
			resultCh <- ErrJoinRoomDenied
		}
	}()

	return <-resultCh
}

func (r *goroutineSafeRoom) Leave(id string) (innerErr error) {
	op, err := r.Enqueue(func() {
		innerErr = r.r.Leave(id)
	})

	if err != nil {
		return err
	}

	<-op.Done()
	return
}

func (r *goroutineSafeRoom) Broadcast(id string, body interface{}) error {
	return r.broadcast(id, body, true)
}

func (r *goroutineSafeRoom) BroadcastWithoutInterception(id string, body interface{}) error {
	return r.broadcast(id, body, false)
}

func (r *goroutineSafeRoom) broadcast(id string, body interface{}, interception bool) error {
	msg := internalMessage{
		kind: internalBroadcastMessage,
		body: internalBroadcastMessageBody{
			userID:       id,
			body:         body,
			interception: interception,
		},
	}
	return r.r.safeSendMessage(msg)
}

func (r *goroutineSafeRoom) CustomMessage(id string, kind CustomMessageKind, body interface{}) error {
	return r.customMessage(id, kind, body, true)
}

func (r *goroutineSafeRoom) CustomMessageWithoutInterception(id string, kind CustomMessageKind, body interface{}) error {
	return r.customMessage(id, kind, body, false)
}

func (r *goroutineSafeRoom) customMessage(id string, kind CustomMessageKind, body interface{}, interception bool) error {
	msg := internalMessage{
		kind: internalCustomMessage,
		body: internalCustomMessageBody{
			userID:       id,
			kind:         kind,
			body:         body,
			interception: interception,
		},
	}
	return r.r.safeSendMessage(msg)
}

func (r *goroutineSafeRoom) Closed() bool {
	return r.r.Closed()
}
