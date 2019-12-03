package main

import (
	"context"
	"log"

	"github.com/yaegaki/hibari"
)

type roomAllocator struct {
}

type roomHandler struct {
	id string
}

func (roomAllocator) Alloc(ctx context.Context, id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		id: id,
	}

	return hibari.NewRoom(id, m, rh, hibari.RoomOption{}), nil
}

func (roomAllocator) Free(string) {
}

func (rh *roomHandler) ValidateJoinUser(ctx context.Context, r hibari.Room, u hibari.User) error {
	return nil
}

func (rh *roomHandler) OnJoinUser(_ hibari.Room, u hibari.InRoomUser) {
	log.Printf("join user: %v(%v)", u.User.Name, u.User.ID)
}

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, u hibari.InRoomUser) {
	log.Printf("leave user: %v(%v)", u.User.Name, u.User.ID)
	roomInfo := r.RoomInfo()
	if len(roomInfo.UserMap) > 0 {
		return
	}

	r.Shutdown()
}

func (rh *roomHandler) OnCustomMessage(r hibari.Room, user hibari.InRoomUser, kind hibari.CustomMessageKind, body interface{}) {
	// disconnect user
	r.Leave(user.User.ID)
}

func (rh *roomHandler) OnShutdown() {
	log.Printf("Shutdown room %v", rh.id)
}
