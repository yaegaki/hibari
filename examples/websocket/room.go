package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/vmihailenco/msgpack/v4"
	"github.com/yaegaki/hibari"
)

type roomAllocator struct {
	db   db
	rule roomRule
}

type roomHandler struct {
	id   string
	db   db
	rule roomRule
}

type roomRule struct {
	maxUser int
}

func (ra *roomAllocator) Alloc(id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		id:   id,
		db:   ra.db,
		rule: ra.rule,
	}

	return hibari.NewRoom(id, m, rh, hibari.RoomOption{
		Deadline: 5 * time.Second,
	}), nil
}

func (rh *roomHandler) Authenticate(ctx context.Context, _ hibari.Room, id, secret string) (hibari.User, error) {
	u, err := rh.db.findUser(id)
	if err != nil {
		return hibari.User{}, err
	}

	if u.secret != secret {
		return hibari.User{}, fmt.Errorf("invalid secret")
	}

	return hibari.User{ID: u.id, Name: u.name}, nil
}

func (rh *roomHandler) ValidateJoinUser(ctx context.Context, r hibari.Room, u hibari.User) error {
	userCount := len(r.RoomInfo().UserMap)
	if userCount >= rh.rule.maxUser {
		return fmt.Errorf("No vacancy")
	}

	return nil
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

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ hibari.InRoomUser) {
	if len(r.RoomInfo().UserMap) > 0 {
		return
	}

	r.Shutdown()
}

func (rh *roomHandler) OnShutdown() {
	log.Printf("Shutdown room %v", rh.id)
}

func (rh *roomHandler) handleRoomInfoMessage(r hibari.Room, user hibari.InRoomUser) {
	conn, err := r.GetConn(user.User.ID)
	if err != nil {
		return
	}

	userMap := map[string]shortUser{}
	for id, u := range r.RoomInfo().UserMap {
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

	conn.OnBroadcast(user, bin)
}

func (rh *roomHandler) handleDiceMessage(r hibari.Room, user hibari.InRoomUser) {

	msg := customMessage{
		Kind: diceMessage,
		Body: diceMessageBody{
			Value: rand.Intn(6),
		},
	}

	bin, err := msgpack.Marshal(msg)
	if err != nil {
		return
	}

	for userID := range r.RoomInfo().UserMap {
		conn, err := r.GetConn(userID)
		if err != nil {
			continue
		}
		conn.OnBroadcast(user, bin)
	}
}
