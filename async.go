package hibari

type asyncOperation struct {
	ch <-chan struct{}
}

// AsyncOperation represents asyncronous operation
type AsyncOperation interface {
	// Done waits finish operation
	Done() <-chan struct{}
}

func (op asyncOperation) Done() <-chan struct{} {
	return op.ch
}

// NewAsyncOperation creates AsyncOperation
func NewAsyncOperation(ch <-chan struct{}) AsyncOperation {
	return asyncOperation{ch: ch}
}
