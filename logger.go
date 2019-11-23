package hibari

import "log"

// Logger is the interface for logging
type Logger interface {
	Printf(format string, v ...interface{})
}

type stdLogger struct{}

func (stdLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}
