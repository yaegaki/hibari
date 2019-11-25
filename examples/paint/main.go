package main

import (
	"net/http"

	"github.com/yaegaki/hibari"
	"github.com/yaegaki/hibari/websocket"
)

func main() {
	manager := hibari.NewManager(roomAllocator{}, nil)

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
