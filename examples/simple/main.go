package main

import (
	"context"
	"fmt"
	"log"
	"sync"
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

func (ra roomAllocator) Alloc(id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		id: id,
		ra: ra,
	}

	return hibari.NewRoom(id, m, rh, hibari.RoomOption{
		Deadline: 1 * time.Second,
	}), nil
}

type roomHandler struct {
	id    string
	mu    *sync.Mutex
	index int
	ra    roomAllocator
}

func (rh *roomHandler) Authenticate(_ context.Context, _ hibari.Room, id, secret string) (hibari.User, error) {
	u, ok := rh.ra.userMap[id]
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

func (rh *roomHandler) ValidateJoinUser(userCtx context.Context, r hibari.Room, u hibari.User) error {
	info := r.RoomInfo()

	if len(info.UserMap) >= rh.ra.rule.maxUser {
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

func (rh *roomHandler) OnCustomMessage(r hibari.Room, _ hibari.InRoomUser, _ interface{}) {
}

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ hibari.InRoomUser) {
	if len(r.RoomInfo().UserMap) == 0 {
		r.Shutdown()
	}
}

func (rh *roomHandler) OnShutdown() {
	log.Printf("Shutdown room: %v", rh.id)
}

type conn struct {
	name string
}

func (c conn) OnAuthenticationFailed() {
	log.Printf("Authentication failed: %v", c.name)
}

func (c conn) OnJoinFailed(err error) {
	log.Printf("Join failed: %v reason: %v", c.name, err)
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
}

func main() {
	ra := roomAllocator{
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
		rule: rule{
			maxUser: 3,
		},
	}
	manager := hibari.NewManager(ra, nil)

	_, err := manager.GetOrCreateRoom("roomC")
	if err != nil {
		panic(err)
	}

	roomA, err := manager.GetOrCreateRoom("roomA")
	if err != nil {
		panic(err)
	}

	roomB, err := manager.GetOrCreateRoom("roomB")
	if err != nil {
		panic(err)
	}

	none := context.Background()
	userCtxA := context.WithValue(none, customValueKey, "helloA")
	userCtxB := context.WithValue(none, customValueKey, "helloB")

	roomA.Join(userCtxA, "test1", "xxx", conn{name: "test1(roomA)"})
	roomA.Join(userCtxB, "test2", "yyy", conn{name: "test2(roomA)"})
	roomA.Join(userCtxA, "test3", "zzz", conn{name: "test3(roomA)"})

	roomA.Join(userCtxB, "test4", "q", conn{name: "test4(roomA)"})
	<-time.After(100 * time.Millisecond)
	roomA.Join(userCtxA, "test4", "qqq", conn{name: "test4(roomA)"})
	<-time.After(100 * time.Millisecond)

	roomB.Join(userCtxB, "test1", "xxx", conn{name: "test1(roomB)"})
	roomB.Join(userCtxA, "test2", "yyy", conn{name: "test2(roomB)"})

	<-time.After(1 * time.Second)
	for roomInfo := range manager.RoomInfoAll() {
		log.Printf("Room: %v", roomInfo.ID)
		for id, u := range roomInfo.UserMap {
			log.Printf("User: %v(%v)", u.User.Name, id)
		}
	}

	roomB.Join(userCtxB, "test3", "zzz", conn{name: "test3(roomB)"})

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
