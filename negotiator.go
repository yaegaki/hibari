package hibari

import "context"

// Negotiator negotiates the room option
type Negotiator interface {
	Negotiate(ctx context.Context, trans ConnTransport) (context.Context, error)
}
