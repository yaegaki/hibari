package hibari

type internalMessage struct {
	kind internalMessageKind
	body interface{}
}

type internalMessageKind int

const (
	internalJoinMessage internalMessageKind = iota
	internalPreLeaveMessage
	internalLeaveMessage
	internalBroadcastMessage
	internalCustomMessage

	internalShutdownMessage
)

type internalJoinMessageBody struct {
	user User
	conn Conn
}

type internalPreLeaveMessageBody struct {
	userID string
}

type internalLeaveMessageBody struct {
	user roomUser
}

type internalBroadcastMessageBody struct {
	userID string
	body   interface{}
}

type internalCustomMessageBody struct {
	user roomUser
}
