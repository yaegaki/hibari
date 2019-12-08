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
	Context() context.Context

	Run()
	Close()
	Enqueue(f func()) (AsyncOperation, error)

	// ForGoroutine creates a goroutine safe room.
	ForGoroutine() GoroutineSafeRoom

	ID() string
	UserCount() int
	RoomInfo() RoomInfo
	ExistsUser(id string) bool
	GetUser(id string) (InRoomUser, error)
	Join(ctx context.Context, user User, conn Conn) error
	JoinWithoutInterception(ctx context.Context, user User, conn Conn) error
	Leave(id string) error
	Broadcast(id string, body interface{}) error
	BroadcastWithoutInterception(id string, body interface{}) error
	CustomMessage(id string, kind CustomMessageKind, body interface{}) error
	CustomMessageWithoutInterception(id string, kind CustomMessageKind, body interface{}) error
	Closed() bool
}

// GoroutineSafeRoom is goroutine safe version room.
type GoroutineSafeRoom interface {
	Room

	SafeUserCount() (int, error)
	SafeRoomInfo() (RoomInfo, error)
}

// RoomOption configures the room.
type RoomOption struct {
	Deadline time.Duration
	// KickFirstComesUser means kicks first user if same user id's user joins the room.
	KickFirstComesUser bool
	Interceptor        RoomInterceptor
	Logger             Logger
}

type room struct {
	ctx    context.Context
	cancel context.CancelFunc

	id           string
	option       RoomOption
	manager      Manager
	handler      RoomHandler
	interceptor  RoomInterceptor
	userMap      map[string]InRoomUser
	msgCh        chan internalMessage
	currentIndex int
	logger       Logger
	closed       bool
	runOnce      *sync.Once
	closeOnce    *sync.Once
}

type goroutineSafeRoom struct {
	r *room
}

// RoomInfo is room information
type RoomInfo struct {
	ID      string
	UserMap map[string]InRoomUser
}

// ErrRoomAlreadyClosed is returned when access to the room when that had already closed.
var ErrRoomAlreadyClosed = errors.New("the room had already closed")

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

// CreateRoomRequest contains info for RoomSuggester.
type CreateRoomRequest struct {
	Context context.Context
	ID      string
}

// RoomSuggestion contains room creation info.
type RoomSuggestion struct {
	ID          string
	RoomHandler RoomHandler
	Option      RoomOption
}

// RoomSuggester suggests a room creation info.
type RoomSuggester interface {
	Suggest(req CreateRoomRequest, m Manager) (RoomSuggestion, error)
}

type internalRoomSuggester struct{}

func (internalRoomSuggester) Suggest(req CreateRoomRequest, m Manager) (RoomSuggestion, error) {
	return RoomSuggestion{
		ID:          req.ID,
		RoomHandler: internalRoomHandler{},
	}, nil
}

// RoomHandler customizes the room behavior.
type RoomHandler interface {
	OnCreate(r Room)
	OnClose(r Room)

	ValidateJoinUser(r Room, user InRoomUser) error

	OnJoinUser(r Room, user InRoomUser)
	OnDisconnectUser(r Room, user InRoomUser)
	OnCustomMessage(r Room, user InRoomUser, kind CustomMessageKind, body interface{})
}

type internalRoomHandler struct{}

func (internalRoomHandler) OnCreate(Room) {
}

func (internalRoomHandler) OnClose(Room) {
}

func (internalRoomHandler) ValidateJoinUser(Room, InRoomUser) error {
	return nil
}

func (internalRoomHandler) OnJoinUser(Room, InRoomUser) {
}

func (internalRoomHandler) OnDisconnectUser(Room, InRoomUser) {
}

func (internalRoomHandler) OnCustomMessage(Room, InRoomUser, CustomMessageKind, interface{}) {
}

func newRoom(id string, m Manager, rh RoomHandler, option RoomOption) *room {
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

	ctx, cancel := context.WithCancel(context.Background())

	r := &room{
		ctx:    ctx,
		cancel: cancel,

		id:          id,
		option:      option,
		manager:     m,
		handler:     rh,
		interceptor: option.Interceptor,
		userMap:     map[string]InRoomUser{},
		msgCh:       make(chan internalMessage),
		logger:      option.Logger,
		runOnce:     &sync.Once{},
		closeOnce:   &sync.Once{},
	}

	return r
}

func (r *room) Context() context.Context {
	return r.ctx
}

func (r *room) Run() {
	r.runOnce.Do(func() {
		t := time.NewTicker(r.option.Deadline)
		defer t.Stop()

		r.handler.OnCreate(r)

		updatedTime := time.Now()
		for {
			select {
			case msg := <-r.msgCh:
				// r.logger.Printf("%v", msg)
				if msg.kind == internalCloseMessage {
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
					r.Close()
				}
			}
		}
	})
}

func (r *room) Close() {
	go func() {
		r.closeOnce.Do(func() {
			msg := internalMessage{
				kind: internalCloseMessage,
			}
			r.msgCh <- msg

			for _, u := range r.userMap {
				u.Conn.Close()
			}

			r.closed = true
			r.cancel()
			r.handler.OnClose(r)

			r.manager.NotifyRoomClosed(r.id)
		})
	}()
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

func (r *room) ForGoroutine() GoroutineSafeRoom {
	return &goroutineSafeRoom{
		r: r,
	}
}

func (r *room) ID() string {
	return r.id
}

func (r *room) UserCount() int {
	return len(r.userMap)
}

func (r *room) RoomInfo() RoomInfo {
	userMap := map[string]InRoomUser{}
	for id, u := range r.userMap {
		userMap[id] = u
	}

	return RoomInfo{
		ID:      r.id,
		UserMap: userMap,
	}
}

func (r *room) ExistsUser(id string) bool {
	_, ok := r.userMap[id]
	return ok
}

func (r *room) GetUser(id string) (InRoomUser, error) {
	u, ok := r.userMap[id]
	if !ok {
		return InRoomUser{}, UserNotFoundError{ID: id}
	}

	return u, nil
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
		if !r.option.KickFirstComesUser {
			return nil, AlreadyInRoomError{User: user}
		}

		r.Leave(user.ID)
	}

	joinedUser := InRoomUser{
		Ctx:   ctx,
		Index: r.currentIndex,
		User:  user,
		Conn:  conn,
	}

	err := r.handler.ValidateJoinUser(r, joinedUser)
	if err != nil {
		return nil, err
	}

	if interception {
		result, pendingCtx := r.interceptor.InterceptJoin(r, joinedUser)
		switch result {
		case JoinInterceptionPending:
			return pendingCtx, ErrJoinRoomPending
		case JoinInterceptionDeny:
			return nil, ErrJoinRoomDenied
		default:
		}
	}

	r.userMap[joinedUser.User.ID] = joinedUser
	r.currentIndex++

	roomInfo := r.RoomInfo()
	err = joinedUser.Conn.OnJoin(roomInfo)

	if err != nil {
		// already user is joined but disconnected. so room must send leave message for other users.
		r.enqueueLeave(joinedUser.User.ID)
	}

	// user join completed.

	for _, u := range r.userMap {
		if u.Index == joinedUser.Index {
			continue
		}

		err = u.Conn.OnOtherUserJoin(joinedUser)
		if err != nil {
			r.enqueueLeave(u.User.ID)
		}
	}

	// Notify joinUser after all message sent
	r.handler.OnJoinUser(r, joinedUser)

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
	inRoomUser, ok := r.userMap[id]
	if !ok {
		return UserNotFoundError{ID: id}
	}

	delete(r.userMap, id)
	inRoomUser.Conn.Close()

	for _, u := range r.userMap {
		err := u.Conn.OnOtherUserLeave(inRoomUser)
		if err != nil {
			r.enqueueLeave(u.User.ID)
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
	inRoomUser, ok := r.userMap[id]
	if !ok {
		return UserNotFoundError{ID: id}
	}

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
		err := u.Conn.OnBroadcast(inRoomUser, body)
		if err != nil {
			r.enqueueLeave(u.User.ID)
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
	inRoomUser, ok := r.userMap[id]
	if !ok {
		return UserNotFoundError{ID: id}
	}

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
	case <-r.ctx.Done():
		return ErrRoomAlreadyClosed
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

func (r *goroutineSafeRoom) Context() context.Context {
	return r.r.Context()
}

func (r *goroutineSafeRoom) Run() {
	r.r.Run()
}

func (r *goroutineSafeRoom) Close() {
	r.r.Close()
}

func (r *goroutineSafeRoom) Enqueue(f func()) (AsyncOperation, error) {
	finishCh := make(chan struct{})
	err := r.r.unsafeEnqueue(finishCh, f)

	if err != nil {
		return nil, err
	}

	return NewAsyncOperation(finishCh), nil
}

func (r *goroutineSafeRoom) ID() string {
	return r.r.ID()
}

func (r *goroutineSafeRoom) ForGoroutine() GoroutineSafeRoom {
	return r
}

func (r *goroutineSafeRoom) UserCount() int {
	userCount, _ := r.SafeUserCount()
	return userCount
}

func (r *goroutineSafeRoom) SafeUserCount() (userCount int, err error) {
	op, err := r.Enqueue(func() {
		userCount = r.r.UserCount()
	})

	if err != nil {
		return 0, err
	}

	<-op.Done()
	return
}

func (r *goroutineSafeRoom) RoomInfo() RoomInfo {
	roomInfo, _ := r.SafeRoomInfo()
	return roomInfo
}

func (r *goroutineSafeRoom) SafeRoomInfo() (roomInfo RoomInfo, err error) {
	op, err := r.Enqueue(func() {
		roomInfo = r.r.RoomInfo()
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

func (r *goroutineSafeRoom) GetUser(id string) (user InRoomUser, innerErr error) {
	op, err := r.Enqueue(func() {
		user, innerErr = r.r.GetUser(id)
	})

	if err != nil {
		return InRoomUser{}, err
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
