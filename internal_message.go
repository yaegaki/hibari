package hibari

import "context"

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

	internalInvokeMessage
	internalShutdownMessage
)

type internalJoinMessageBody struct {
	ctx  context.Context
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

type internalInvokeMessageBody struct {
	f func()
}

type internalCustomMessageBody struct {
	userID string
	kind   CustomMessageKind
	body   interface{}
}
