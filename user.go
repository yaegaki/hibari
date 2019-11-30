package hibari

import "context"

// User is user
type User struct {
	ID   string
	Name string
}

// InRoomUser is user in the room
type InRoomUser struct {
	Ctx   context.Context
	Index int
	User  User
}
