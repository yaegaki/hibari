package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yaegaki/hibari"
)

type roomAllocator struct {
	db   db
	rule roomRule
}

type roomHandler struct {
	db   db
	rule roomRule
}

type roomRule struct {
	maxUser int
}

func (ra *roomAllocator) Alloc(id string, m hibari.Manager) (hibari.Room, error) {
	rh := &roomHandler{
		db:   ra.db,
		rule: ra.rule,
	}

	return hibari.NewRoom(id, m, rh, nil), nil
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

func (rh *roomHandler) OnDisconnectUser(r hibari.Room, _ string) {
	if len(r.RoomInfo().UserMap) > 0 {
		return
	}

	log.Printf("Shutdown room %v", r.ID())
	r.Shutdown()
}
