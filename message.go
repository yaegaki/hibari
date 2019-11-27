package hibari

// Message .
type Message struct {
	Kind MessageKind
	Body interface{}
}

// MessageKind indicates message body type
type MessageKind int

const (
	// ClientToServer

	// JoinMessage is message of join room request
	JoinMessage MessageKind = 1
	// BroadcastMessage is message of user's broadcast request
	BroadcastMessage MessageKind = 2
	// CustomMessage is message of user's custom message request
	CustomMessage MessageKind = 3

	// ServerToClinet

	// OnAuthenticationFailedMessage is message of authentication failed
	OnAuthenticationFailedMessage MessageKind = 11
	// OnJoinFailedMessage is message of join room failed
	OnJoinFailedMessage MessageKind = 12
	// OnJoinMessage is message of join room
	OnJoinMessage MessageKind = 13
	// OnOtherUserJoinMessage is message of other user's join room
	OnOtherUserJoinMessage MessageKind = 14
	// OnOtherUserLeaveMessage is message of other user's leave room
	OnOtherUserLeaveMessage MessageKind = 15
	// OnBroadcastMessage is message of broadcast
	OnBroadcastMessage MessageKind = 16
)

// AnyMessageEncoderDecoder encode/decode brodacast/custom message body to connTransport independent type
type AnyMessageEncoderDecoder interface {
	// EncodeAnyMessageBody convert ConnTransport dependent body to independent body
	EncodeAnyMessageBody(body interface{}) (interface{}, error)
	// DecodeAnyMessageBody convert ConnTransport independent body to dependent body
	DecodeAnyMessageBody(body interface{}) (interface{}, error)
}

// JoinMessageBody .
type JoinMessageBody struct {
	UserID string
	Secret string
	RoomID string
}

// BroadcastMessageBody .
type BroadcastMessageBody interface{}

// CustomMessageBody .
type CustomMessageBody struct {
	Kind CustomMessageKind
	Body interface{}
}

// CustomMessageKind indicates custom message body type
type CustomMessageKind int

// ShortUser is part of User
type ShortUser struct {
	Index int
	Name  string
}

// OnJoinMessageBody .
type OnJoinMessageBody struct {
	UserMap map[string]ShortUser
}

// OnOtherUserJoinMessageBody .
type OnOtherUserJoinMessageBody struct {
	User ShortUser
}

// OnOtherUserLeaveMessageBody .
type OnOtherUserLeaveMessageBody struct {
	User ShortUser
}

// OnBroadcastMessageBody .
type OnBroadcastMessageBody struct {
	From ShortUser
	Body interface{}
}

// InvalidMessageError is occurred if create message by invalid body type
type InvalidMessageError string

func (e InvalidMessageError) Error() string {
	return string(e)
}

// NewMessage creates new message
func NewMessage(body interface{}) (Message, error) {
	var kind MessageKind
	switch body.(type) {
	case JoinMessageBody:
		kind = JoinMessage
	// case BroadcastMessageBody:
	// 	kind = BroadcastMessage
	case CustomMessageBody:
		kind = CustomMessage

	case OnJoinMessageBody:
		kind = OnJoinMessage
	case OnOtherUserJoinMessageBody:
		kind = OnOtherUserJoinMessage
	case OnOtherUserLeaveMessageBody:
		kind = OnOtherUserLeaveMessage
	case OnBroadcastMessageBody:
		kind = OnBroadcastMessage

	default:
		return Message{}, InvalidMessageError("InvalidMessageType")
	}

	return Message{Kind: kind, Body: body}, nil
}
