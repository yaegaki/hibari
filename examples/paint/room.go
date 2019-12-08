package main

import (
	"log"

	"github.com/yaegaki/hibari"
)

type roomSuggester struct {
}

type roomHandler struct {
}

func (roomSuggester) Suggest(req hibari.CreateRoomRequest, m hibari.Manager) (hibari.RoomSuggestion, error) {
	return hibari.RoomSuggestion{
		ID:          req.ID,
		RoomHandler: roomHandler{},
	}, nil
}

func (roomHandler) OnCreate(r hibari.Room) {
	log.Printf("Create room %v", r.ID())
}

func (roomHandler) ValidateJoinUser(r hibari.Room, u hibari.InRoomUser) error {
	return nil
}

func (roomHandler) OnJoinUser(_ hibari.Room, _ hibari.InRoomUser) {
}

func (roomHandler) OnDisconnectUser(r hibari.Room, _ hibari.InRoomUser) {
	roomInfo := r.RoomInfo()
	if len(roomInfo.UserMap) > 0 {
		return
	}

	r.Close()
}

func (roomHandler) OnCustomMessage(r hibari.Room, user hibari.InRoomUser, kind hibari.CustomMessageKind, body interface{}) {
	// disconnect user
	r.Leave(user.User.ID)
}

func (roomHandler) OnClose(r hibari.Room) {
	log.Printf("Close room %v", r.ID())
}
