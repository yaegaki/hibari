package hibari

import "context"

// Authenticator authenticate user
type Authenticator interface {
	Authenticate(ctx context.Context, id, secret string) (User, error)
}
