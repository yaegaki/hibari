package main

import (
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

func (ra roomAllocator) Alloc(id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		id: id,
		ra: ra,
	}

	return hibari.NewRoom(id, m, rh, nil), nil
}

type roomHandler struct {
	id    string
	mu    *sync.Mutex
	index int
	ra    roomAllocator
}

func (rh *roomHandler) Authenticate(_ hibari.Room, id, secret string) (hibari.User, error) {
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

func (rh *roomHandler) ValidateJoinUser(r hibari.Room, u hibari.User) error {
	info := r.RoomInfo()

	if len(info.UserMap) >= rh.ra.rule.maxUser {
		return fmt.Errorf("No vacancy")
	}

	return nil
}

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ string) {
	if len(r.RoomInfo().UserMap) == 0 {
		r.Shutdown()
	}
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
	log.Printf("%v: other user join! %v", c.name, u.Name)
	return nil
}

func (c conn) OnOtherUserLeave(u hibari.InRoomUser) error {
	log.Printf("%v: other user leave! %v", c.name, u.Name)
	return nil
}

func (c conn) OnBroadcast(from hibari.InRoomUser, body interface{}) error {
	log.Printf("%v: message %v from %v", c.name, body, from.Name)
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

	roomA, err := manager.GetOrCreateRoom("roomA")
	if err != nil {
		panic(err)
	}

	roomB, err := manager.GetOrCreateRoom("roomB")
	if err != nil {
		panic(err)
	}

	roomA.Join("test1", "xxx", conn{name: "test1(roomA)"})
	roomA.Join("test2", "yyy", conn{name: "test2(roomA)"})
	roomA.Join("test3", "zzz", conn{name: "test3(roomA)"})

	roomA.Join("test4", "q", conn{name: "test4(roomA)"})
	<-time.After(100 * time.Millisecond)
	roomA.Join("test4", "qqq", conn{name: "test4(roomA)"})
	<-time.After(100 * time.Millisecond)

	roomB.Join("test1", "xxx", conn{name: "test1(roomB)"})
	roomB.Join("test2", "yyy", conn{name: "test2(roomB)"})
	roomB.Join("test3", "zzz", conn{name: "test3(roomB)"})

	roomA.Broadcast("test1", "hello!1")
	<-time.After(100 * time.Millisecond)
	roomB.Broadcast("test2", "hello!2")
	<-time.After(100 * time.Millisecond)
	roomA.Leave("test2")
	<-time.After(100 * time.Millisecond)
	roomB.Broadcast("test2", "hello!3")
	<-time.After(100 * time.Millisecond)
	manager.Shutdown()
}
