package main

import (
	"net/http"
	"sync"

	"github.com/yaegaki/hibari"
	"github.com/yaegaki/hibari/websocket"
)

func main() {
	manager := hibari.NewManager(roomAllocator{}, &hibari.ManagerOption{
		Authenticator: authenticator{
			mu: &sync.Mutex{},
		},
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/main.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "main.js")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(manager, w, r)
	})

	http.ListenAndServe(":23032", nil)
}
