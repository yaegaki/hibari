package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/vmihailenco/msgpack/v4"
	"github.com/yaegaki/hibari"
)

type roomSuggester struct {
	rule roomRule
}

type roomHandler struct {
	rule roomRule
}

type roomRule struct {
	maxUser int
}

func (rs *roomSuggester) Suggest(req hibari.CreateRoomRequest, m hibari.Manager) (hibari.RoomSuggestion, error) {
	rh := &roomHandler{
		rule: rs.rule,
	}

	return hibari.RoomSuggestion{
		ID:          req.ID,
		RoomHandler: rh,
		Option: hibari.RoomOption{
			Deadline: 5 * time.Second,
		},
	}, nil
}

func (rh *roomHandler) OnCreate(hibari.Room) {
}

func (rh *roomHandler) ValidateJoinUser(r hibari.Room, u hibari.InRoomUser) error {
	roomInfo := r.RoomInfo()
	userCount := len(roomInfo.UserMap)
	if userCount >= rh.rule.maxUser {
		return fmt.Errorf("No vacancy")
	}

	return nil
}

func (rh *roomHandler) OnJoinUser(_ hibari.Room, _ hibari.InRoomUser) {
}

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ hibari.InRoomUser) {
	roomInfo := r.RoomInfo()
	if len(roomInfo.UserMap) > 0 {
		return
	}

	r.Close()
}

func (rh *roomHandler) OnCustomMessage(r hibari.Room, user hibari.InRoomUser, kind hibari.CustomMessageKind, body interface{}) {
	switch kind {
	case roomInfoMessage:
		rh.handleRoomInfoMessage(r, user)
	case diceMessage:
		rh.handleDiceMessage(r, user)
	default:
	}
}

func (rh *roomHandler) OnClose(r hibari.Room) {
	log.Printf("Close room %v", r.ID())
}

func (rh *roomHandler) handleRoomInfoMessage(r hibari.Room, user hibari.InRoomUser) {
	userMap := map[string]shortUser{}
	roomInfo := r.RoomInfo()
	for id, u := range roomInfo.UserMap {
		userMap[id] = shortUser{
			Index: u.Index,
			Name:  u.User.Name,
		}
	}

	msg := customMessage{
		Kind: roomInfoMessage,
		Body: roomInfoMessageBody{
			UserMap: userMap,
		},
	}
	bin, err := msgpack.Marshal(msg)
	if err != nil {
		return
	}

	if err = user.Conn.OnBroadcast(user, bin); err != nil {
		user.Conn.Close()
	}
}

func (rh *roomHandler) handleDiceMessage(r hibari.Room, user hibari.InRoomUser) {

	msg := customMessage{
		Kind: diceMessage,
		Body: diceMessageBody{
			User: shortUser{
				Index: user.Index,
				Name:  user.User.Name,
			},
			Value: rand.Intn(6),
		},
	}

	bin, err := msgpack.Marshal(msg)
	if err != nil {
		return
	}

	sys := hibari.InRoomUser{
		Index: -1,
		User: hibari.User{
			ID:   "system",
			Name: "system",
		},
	}

	roomInfo := r.RoomInfo()
	for userID := range roomInfo.UserMap {
		u, err := r.GetUser(userID)
		if err != nil {
			continue
		}

		if err = u.Conn.OnBroadcast(sys, bin); err != nil {
			u.Conn.Close()
		}
	}
}
