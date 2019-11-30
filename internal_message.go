package hibari

type internalMessage struct {
	kind internalMessageKind
	body interface{}
}

type internalMessageKind int

const (
	internalBroadcastMessage internalMessageKind = iota
	internalCustomMessage

	internalInvokeMessage
	internalShutdownMessage
)

type internalInvokeMessageBody struct {
	f func()
}

type internalBroadcastMessageBody struct {
	userID       string
	body         interface{}
	interception bool
}

type internalCustomMessageBody struct {
	userID       string
	kind         CustomMessageKind
	body         interface{}
	interception bool
}
