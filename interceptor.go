package hibari

import (
	"context"
)

// RoomInterceptor intercept join/broadcast/custom message.
type RoomInterceptor interface {
	InterceptJoin(ctx context.Context, room Room, user User, conn Conn) (JoinInterceptionResult, context.Context)
	InterceptBroadcast(room Room, user InRoomUser, body interface{}) MessageInterceptionResult
	InterceptCustomMessage(room Room, user InRoomUser, kind CustomMessageKind, body interface{}) MessageInterceptionResult
}

// JoinInterceptionResult reprecents InterceptJoin's result.
type JoinInterceptionResult int

const (
	// JoinInterceptionAllow represents join request was allowed.
	JoinInterceptionAllow JoinInterceptionResult = iota
	// JoinInterceptionPending represents join request is pending.
	JoinInterceptionPending
	// JoinInterceptionDeny represents join request was denied.
	JoinInterceptionDeny
)

// MessageInterceptionResult reprecents InterceptBroadcast/CustomMessage result.
type MessageInterceptionResult int

const (
	// MessageInterceptionAllow represents message was allowed.
	MessageInterceptionAllow MessageInterceptionResult = iota
	// MessageInterceptionDeny represents message was Denied.
	MessageInterceptionDeny
	// MessageInterceptionDisconnect represents message was Denied and sending user was disconnected.
	MessageInterceptionDisconnect
)

// NilRoomInterceptor is implements RoomInterceptor.
type NilRoomInterceptor struct {
}

// InterceptJoin is always return JoinInterceptionAllow.
func (NilRoomInterceptor) InterceptJoin(ctx context.Context, room Room, user User, conn Conn) (JoinInterceptionResult, context.Context) {
	return JoinInterceptionAllow, nil
}

// InterceptBroadcast is always return MessageInterceptionAllow.
func (NilRoomInterceptor) InterceptBroadcast(room Room, user InRoomUser, body interface{}) MessageInterceptionResult {
	return MessageInterceptionAllow
}

// InterceptCustomMessage is always return MessageInterceptionAllow.
func (NilRoomInterceptor) InterceptCustomMessage(room Room, user InRoomUser, kind CustomMessageKind, body interface{}) MessageInterceptionResult {
	return MessageInterceptionAllow
}
