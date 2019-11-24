package main

type customMessage struct {
	Kind customMessageKind `msgpack:"kind"`
	Body interface{}       `msgpack:"body"`
}

type customMessageKind int

const (
	chatMessage customMessageKind = iota
	roomInfoMessage
	diceMessage
)

type inRoomUser struct {
	Index int    `msgpack:"index"`
	Name  string `msgpack:"name"`
}

type roomInfoMessageBody struct {
	UserMap map[string]inRoomUser `msgpack:"users"`
}

type diceMessageBody struct {
	Value int `msgpack:"value"`
}
