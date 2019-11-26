package main

import "github.com/yaegaki/hibari"

type customMessage struct {
	Kind hibari.CustomMessageKind `msgpack:"kind"`
	Body interface{}              `msgpack:"body"`
}

const (
	chatMessage hibari.CustomMessageKind = iota + 1
	roomInfoMessage
	diceMessage
)

type shortUser struct {
	Index int    `msgpack:"index"`
	Name  string `msgpack:"name"`
}

type roomInfoMessageBody struct {
	UserMap map[string]shortUser `msgpack:"users"`
}

type diceMessageBody struct {
	User  shortUser `msgpack:"user"`
	Value int       `msgpack:"value"`
}
