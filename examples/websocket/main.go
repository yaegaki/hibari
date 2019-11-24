package main

import (
	"fmt"
	"net/http"
	"strings"

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

	db.registerUser("admin-a", "aaa", "★admin-a")
	db.registerUser("admin-b", "bbb", "★admin-b")
	db.registerUser("admin-c", "ccc", "★admin-c")
	db.registerUser("admin-d", "ddd", "★admin-d")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.FormValue("userId")
		password := r.FormValue("password")
		if len(userID) == 0 || len(password) == 0 {
			http.Error(w, "BadRequest", http.StatusBadRequest)
			return
		}

		if strings.Contains(userID, "★") {
			http.Error(w, "BadRequest", http.StatusBadRequest)
			return
		}

		err := db.registerUser(userID, password, userID)
		if err != nil {
			http.Error(w, "Can not register", http.StatusBadRequest)
			return
		}

		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(manager, w, r)
	})

	http.ListenAndServe(":23032", nil)
}
