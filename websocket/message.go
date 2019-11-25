package websocket

import (
	"bytes"
	"fmt"

	"github.com/vmihailenco/msgpack"
	"github.com/yaegaki/hibari"
)

type message struct {
	Kind hibari.MessageKind `msgpack:"kind"`
	Body interface{}        `msgpack:"body,omitempty"`
}

func newMessage(kind hibari.MessageKind, body interface{}) message {
	return message{
		Kind: kind,
		Body: body,
	}
}

type joinMessageBody struct {
	UserID string `msgpack:"userId"`
	Secret string `msgpack:"secret"`
	RoomID string `msgpack:"roomId"`
}

type broadcastMessageBody = []byte

type customMessageBody struct {
	Kind hibari.CustomMessageKind `msgpack:"kind"`
	Body []byte                   `msgpack:"body"`
}

type shortUser struct {
	Index int    `msgpack:"index"`
	Name  string `msgpack:"name"`
}

type onJoinMessageBody struct {
	UserMap map[string]shortUser `msgpack:"users"`
}

type onOtherUserJoinMessageBody struct {
	User shortUser `msgpack:"user"`
}

type onOtherUserLeaveMessageBody struct {
	User shortUser `msgpack:"user"`
}

type onBroadcastMessageBody struct {
	From shortUser `msgpack:"from"`
	Body []byte    `msgpack:"body"`
}

func userMapFrom(userMap map[string]hibari.ShortUser) map[string]shortUser {
	m := map[string]shortUser{}
	for k, v := range userMap {
		m[k] = shortUserFrom(v)
	}
	return m
}

func shortUserFrom(u hibari.ShortUser) shortUser {
	return shortUser{
		Index: u.Index,
		Name:  u.Name,
	}
}

func (msg *message) UnmarshalMsgpack(b []byte) error {
	d := msgpack.NewDecoder(bytes.NewReader(b))
	l, err := d.DecodeMapLen()
	if err != nil {
		return err
	}

	kind := -1
	needReset := false
	read := 0
	for i := 0; i < l; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}

		if key != "kind" {
			if key == "body" {
				needReset = true
			}

			err = d.Skip()
			if err != nil {
				return err
			}

			continue
		}

		k, err := d.DecodeInt()
		if err != nil {
			return err
		}

		kind = k
		read = i + 1
		break
	}

	if kind < 0 {
		return fmt.Errorf("invalid kind for message")
	}

	var decode func() error
	switch (hibari.MessageKind)(kind) {
	case hibari.JoinMessage:
		decode = func() error {
			var b joinMessageBody
			err := d.Decode(&b)
			if err != nil {
				return err
			}
			msg.Body = b
			return nil
		}

	case hibari.BroadcastMessage:
		decode = func() error {
			bytes, err := d.DecodeBytes()
			if err != nil {
				return err
			}
			msg.Body = bytes
			return nil
		}

	case hibari.CustomMessage:
		decode = func() error {
			var b customMessageBody
			err := d.Decode(&b)
			if err != nil {
				return err
			}
			msg.Body = b
			return nil
		}

	default:
		return fmt.Errorf("invalid kind for message")
	}

	if needReset {
		d.Reset(bytes.NewReader(b))
		d.DecodeMapLen()
		read = 0
	}

	for i := read; i < l; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}

		if key != "body" {
			err = d.Skip()
			if err != nil {
				return err
			}

			continue
		}

		err = decode()
		if err != nil {
			return err
		}

		msg.Kind = (hibari.MessageKind)(kind)
		return nil
	}

	return fmt.Errorf("not found body for msg")
}
