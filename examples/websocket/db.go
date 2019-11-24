package main

import (
	"fmt"
	"sync"
)

type db struct {
	mu      *sync.Mutex
	userMap map[string]user
}

func newDB() db {
	return db{
		mu:      &sync.Mutex{},
		userMap: map[string]user{},
	}
}

func (db *db) registerUser(id, secret, name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, ok := db.userMap[id]; ok {
		return fmt.Errorf("already registered")
	}

	db.userMap[id] = user{
		id:     id,
		secret: secret,
		name:   name,
	}

	return nil
}

func (db *db) findUser(id string) (user, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	u, ok := db.userMap[id]
	if !ok {
		return user{}, fmt.Errorf("not found")
	}

	return u, nil
}
