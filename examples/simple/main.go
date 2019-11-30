package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yaegaki/hibari"
)

type user struct {
	id     string
	secret string
	name   string
}

type rule struct {
	maxUser int
}

type roomAllocator struct {
	userMap map[string]user
	rule    rule
}

type key int

const (
	customValueKey key = iota
)

func (ra roomAllocator) Alloc(ctx context.Context, id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		id:   id,
		rule: ra.rule,
	}

	return hibari.NewRoom(id, m, rh, hibari.RoomOption{
		Deadline:    1 * time.Second,
		Interceptor: interceptor{},
	}), nil
}

func (roomAllocator) Free(string) {
}

type authenticator struct {
	userMap map[string]user
}

func (a authenticator) Authenticate(_ context.Context, id, secret string) (hibari.User, error) {
	u, ok := a.userMap[id]
	if !ok {
		return hibari.User{}, fmt.Errorf("Not found user: %v", id)
	}

	if u.secret != secret {
		return hibari.User{}, fmt.Errorf("Invalid secret key: %v", id)
	}

	return hibari.User{
		ID:   id,
		Name: u.name,
	}, nil
}

type roomHandler struct {
	id   string
	rule rule
}

func (rh *roomHandler) ValidateJoinUser(userCtx context.Context, r hibari.Room, u hibari.User) error {
	roomInfo, _ := r.RoomInfo()

	if len(roomInfo.UserMap) >= rh.rule.maxUser {
		return fmt.Errorf("No vacancy")
	}

	customValue := userCtx.Value("custom")
	if customValue != nil {
		v, ok := customValue.(string)
		if ok {
			log.Printf("%v CustomValue: %v", u.ID, v)
		}
	}

	return nil
}

func (rh *roomHandler) OnJoinUser(_ hibari.Room, _ hibari.InRoomUser) {
}

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ hibari.InRoomUser) {
	roomInfo, _ := r.RoomInfo()
	if len(roomInfo.UserMap) == 0 {
		r.Shutdown()
	}
}

func (rh *roomHandler) OnCustomMessage(r hibari.Room, _ hibari.InRoomUser, _ hibari.CustomMessageKind, _ interface{}) {
}

func (rh *roomHandler) OnShutdown() {
	log.Printf("Shutdown room: %v", rh.id)
}

type conn struct {
	ctx    context.Context
	cancel context.CancelFunc
	name   string
}

func newConn(name string) conn {
	ctx, cancel := context.WithCancel(context.Background())
	return conn{
		ctx:    ctx,
		cancel: cancel,
		name:   name,
	}
}

func (c conn) Context() context.Context {
	return c.ctx
}

func (c conn) OnAuthenticationFailed() {
	log.Printf("Authentication failed: %v", c.name)
}

func (c conn) OnJoin(r hibari.RoomInfo) error {
	log.Printf("%v: joinRoom!", c.name)
	return nil
}

func (c conn) OnOtherUserJoin(u hibari.InRoomUser) error {
	log.Printf("%v: other user join! %v", c.name, u.User.Name)
	return nil
}

func (c conn) OnOtherUserLeave(u hibari.InRoomUser) error {
	log.Printf("%v: other user leave! %v", c.name, u.User.Name)
	return nil
}

func (c conn) OnBroadcast(from hibari.InRoomUser, body interface{}) error {
	log.Printf("%v: message %v from %v", c.name, body, from.User.Name)
	return nil
}

func (c conn) Close() {
	c.cancel()
}

type interceptor struct {
}

func (interceptor) InterceptJoin(ctx context.Context, r hibari.Room, user hibari.User, conn hibari.Conn) (hibari.JoinInterceptionResult, context.Context) {
	roomID := r.ID()
	if roomID == "roomA" && user.ID == "test2" {
		log.Printf("Pending:%v", user.ID)
		pendingCtx, cancel := context.WithCancel(context.Background())
		go func() {
			<-time.After(3 * time.Second)
			r.ForGoroutine().JoinWithoutInterception(ctx, user, conn)
			cancel()
		}()
		return hibari.JoinInterceptionPending, pendingCtx
	}

	return hibari.JoinInterceptionAllow, nil
}

func (interceptor) InterceptBroadcast(r hibari.Room, user hibari.InRoomUser, body interface{}) hibari.MessageInterceptionResult {
	return hibari.MessageInterceptionAllow
}

func (interceptor) InterceptCustomMessage(r hibari.Room, user hibari.InRoomUser, kind hibari.CustomMessageKind, body interface{}) hibari.MessageInterceptionResult {
	return hibari.MessageInterceptionAllow
}

func main() {
	ra := roomAllocator{
		rule: rule{
			maxUser: 3,
		},
	}
	manager := hibari.NewManager(ra, &hibari.ManagerOption{
		Authenticator: authenticator{
			userMap: map[string]user{
				"test1": user{
					id:     "test1",
					secret: "xxx",
					name:   "test-user1",
				},
				"test2": user{
					id:     "test2",
					secret: "yyy",
					name:   "test-user2",
				},
				"test3": user{
					id:     "test3",
					secret: "zzz",
					name:   "test-user3",
				},
				"test4": user{
					id:     "test4",
					secret: "qqq",
					name:   "test-user4",
				},
			},
		},
	})

	ctx := context.Background()
	_, err := manager.GetOrCreateRoom(ctx, "roomC")
	if err != nil {
		panic(err)
	}

	roomA, err := manager.GetOrCreateRoom(ctx, "roomA")
	if err != nil {
		panic(err)
	}

	roomB, err := manager.GetOrCreateRoom(ctx, "roomB")
	if err != nil {
		panic(err)
	}

	none := context.Background()
	userCtxA := context.WithValue(none, customValueKey, "helloA")
	userCtxB := context.WithValue(none, customValueKey, "helloB")

	joinRoom := func(ctx context.Context, r hibari.Room, roomName string, id, secret string) {
		user, err := manager.Authenticate(ctx, id, secret)
		if err != nil {
			log.Printf("room: %v authentication failed: %v", roomName, id)
			return
		}

		err = r.Join(ctx, user, newConn(fmt.Sprintf("%v(%v)", user.Name, roomName)))
		if err != nil {
			log.Printf("room: %v join failed: %v(%v)", roomName, user.Name, user.ID)
		}
	}

	joinRoomA := func(ctx context.Context, id, secret string) {
		joinRoom(ctx, roomA, "roomA", id, secret)
	}

	joinRoomB := func(ctx context.Context, id, secret string) {
		joinRoom(ctx, roomB, "roomB", id, secret)
	}

	joinRoomA(userCtxA, "test1", "xxx")
	joinRoomB(userCtxB, "test1", "xxx")

	joinRoomA(userCtxB, "test2", "yyy")
	joinRoomA(userCtxA, "test3", "zzz")

	joinRoomA(userCtxB, "test4", "q")
	<-time.After(100 * time.Millisecond)
	joinRoomA(userCtxA, "test4", "qqq")
	<-time.After(100 * time.Millisecond)

	joinRoomB(userCtxA, "test2", "yyy")

	<-time.After(1 * time.Second)
	for roomInfo := range manager.RoomInfoAll() {
		log.Printf("Room: %v", roomInfo.ID)
		for id, u := range roomInfo.UserMap {
			log.Printf("User: %v(%v)", u.User.Name, id)
		}
	}

	joinRoomB(userCtxB, "test3", "zzz")

	roomA.Broadcast("test1", "hello!1")
	<-time.After(100 * time.Millisecond)
	roomB.Broadcast("test2", "hello!2")
	<-time.After(100 * time.Millisecond)
	roomA.Leave("test2")
	<-time.After(100 * time.Millisecond)
	roomB.Broadcast("test2", "hello!3")
	<-time.After(100 * time.Millisecond)
	manager.Shutdown()
	<-time.After(100 * time.Millisecond)
}
