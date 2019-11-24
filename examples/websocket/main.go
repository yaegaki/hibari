package main

import (
	"net/http"

	"github.com/yaegaki/hibari"
	"github.com/yaegaki/hibari/websocket"
)

func main() {
	db := newDB()
	ra := &roomAllocator{
		db: db,
		rule: roomRule{
			maxUser: 10,
		},
	}
	manager := hibari.NewManager(ra, nil)

	db.registerUser("test-a", "aaa", "test-user-a")
	db.registerUser("test-b", "bbb", "test-user-b")
	db.registerUser("test-c", "ccc", "test-user-c")
	db.registerUser("test-d", "ddd", "test-user-d")

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(manager, w, r)
	})

	http.ListenAndServe(":23032", nil)
}
