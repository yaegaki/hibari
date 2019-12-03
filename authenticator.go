package hibari

import "context"

// Authenticator authenticates a user.
type Authenticator interface {
	Authenticate(ctx context.Context, id, secret string) (User, error)
}
