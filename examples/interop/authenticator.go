package main

import (
	"context"
	"fmt"
	"sync"
	"unicode/utf8"

	"github.com/yaegaki/hibari"
)

type authenticator struct {
	mu *sync.Mutex
}

func (ah authenticator) Authenticate(ctx context.Context, id, secret string) (hibari.User, error) {
	ah.mu.Lock()
	defer ah.mu.Unlock()

	if utf8.RuneCountInString(id) > 8 {
		return hibari.User{}, fmt.Errorf("Too long id")
	}

	return hibari.User{ID: id, Name: id}, nil
}
