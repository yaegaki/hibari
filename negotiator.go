package hibari

import (
	"context"
	"errors"
	"fmt"
)

// NegotiationResult represents Negotiator.Negotiate() results.
type NegotiationResult struct {
	Context context.Context
	User    User
	RoomID  string
}

// Negotiator negotiates about connection.
type Negotiator interface {
	Negotiate(trans ConnTransport, manager Manager) (NegotiationResult, error)
}

// DefaultNegotiator is default implements of Negotiator.
type DefaultNegotiator struct {
}

// ErrAuthenticationFailed is returned when user authentication failed.
var ErrAuthenticationFailed = errors.New("authentication failed")

// Negotiate negotiates about connection.
func (DefaultNegotiator) Negotiate(trans ConnTransport, manager Manager) (NegotiationResult, error) {
	msg, err := trans.ReadMessage()
	if err != nil {
		return NegotiationResult{}, err
	}

	if msg.Kind != JoinMessage {
		return NegotiationResult{}, fmt.Errorf("invalid message kind when join room: %v", msg.Kind)
	}

	body, ok := msg.Body.(JoinMessageBody)
	if !ok {
		return NegotiationResult{}, errors.New("invalid JoinMessageBody")
	}

	ctx := trans.Context()
	user, err := manager.Authenticate(ctx, body.UserID, body.Secret)
	if err != nil {
		return NegotiationResult{}, ErrAuthenticationFailed
	}

	return NegotiationResult{
		Context: ctx,
		User:    user,
		RoomID:  body.RoomID,
	}, nil
}
