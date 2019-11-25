package hibari

import "context"

// ContextConfiger config context before get or create room
type ContextConfiger interface {
	Config(ctx context.Context, trans ConnTransport) (context.Context, error)
}
