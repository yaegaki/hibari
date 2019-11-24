package websocket

type message struct {
	Kind messageKind `msgpack:"kind"`
	Body interface{} `msgpack:"body,omitempty"`
}

type clientToServerMessage struct {
	Kind messageKind `msgpack:"kind"`
	Body []byte      `msgpack:"body,omitempty"`
}

type messageKind int

const (
	joinMessage messageKind = iota
	broadcastMessage

	onAuthenticationFailedMessage
	onJoinFailedMessage
	onJoinMessage
	onOtherUserJoinMessage
	onOtherUserLeaveMessage
	onBroadcastMessage
)

func newMessage(kind messageKind, body interface{}) message {
	return message{
		Kind: kind,
		Body: body,
	}
}

type inRoomUser struct {
	Index int    `msgpack:"index"`
	Name  string `msgpack:"name"`
}

type joinMessageBody struct {
	UserID string `msgpack:"userId"`
	Secret string `msgpack:"secret"`
	RoomID string `msgpack:"roomId"`
}

type onJoinMessageBody struct {
	UserMap map[string]inRoomUser `msgpack:"users"`
}

type otherUserJoinMessageBody struct {
	User inRoomUser `msgpack:"user"`
}

type otherUserLeaveMessageBody struct {
	User inRoomUser `msgpack:"user"`
}

type onBroadcastMessageBody struct {
	From inRoomUser  `msgpack:"from"`
	Body interface{} `msgpack:"body"`
}
