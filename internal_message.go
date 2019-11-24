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
	user    User
	conn    Conn
	userCtx context.Context
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
	body   interface{}
}
