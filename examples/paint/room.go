package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"unicode/utf8"

	"github.com/yaegaki/hibari"
)

type roomAllocator struct {
}

type roomHandler struct {
	id      string
	mu      *sync.Mutex
	userMap map[string]bool
}

func (roomAllocator) Alloc(id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		id: id,
		mu: &sync.Mutex{},
	}

	return hibari.NewRoom(id, m, rh, hibari.RoomOption{}), nil
}

func (rh *roomHandler) Authenticate(ctx context.Context, _ hibari.Room, id, secret string) (hibari.User, error) {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	if utf8.RuneCountInString(id) > 8 {
		return hibari.User{}, fmt.Errorf("Too long id")
	}

	_, ok := rh.userMap[id]
	if ok {
		return hibari.User{}, fmt.Errorf("Already logined")
	}

	return hibari.User{ID: id, Name: id}, nil
}

func (rh *roomHandler) ValidateJoinUser(ctx context.Context, r hibari.Room, u hibari.User) error {
	return nil
}

func (rh *roomHandler) OnCustomMessage(r hibari.Room, user hibari.InRoomUser, kind hibari.CustomMessageKind, body interface{}) {
	// disconnect user
	go r.Leave(user.User.ID)
}

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ hibari.InRoomUser) {
	if len(r.RoomInfo().UserMap) > 0 {
		return
	}

	r.Shutdown()
}

func (rh *roomHandler) OnShutdown() {
	log.Printf("Shutdown room %v", rh.id)
}
