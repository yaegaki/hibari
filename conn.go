package hibari

// Conn represents connection between server and client
type Conn interface {
	OnAuthenticationFailed()
	OnJoinFailed(error)

	OnJoin(r RoomInfo) error
	OnOtherUserJoin(u InRoomUser) error
	OnOtherUserLeave(u InRoomUser) error
	OnBroadcast(from InRoomUser, body interface{}) error
	Close()
}
