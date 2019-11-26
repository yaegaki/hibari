package hibari

import "context"

// Negotiator negotiate room option
type Negotiator interface {
	Negotiate(ctx context.Context, trans ConnTransport) (context.Context, error)
}
