package turtleware

import (
	"github.com/rs/zerolog"
)

// OpenTracingZerologLogger is an implementation of the openTracing Logger interface
// that delegates to a given Zerolog logger. If the logger is nil, nothing will be logged
// out.
type OpenTracingZerologLogger struct {
	logger *zerolog.Logger
}

// Error logs an error message as an Zerolog Errorf.
func (l OpenTracingZerologLogger) Error(msg string) {
	if l.logger == nil {
		return
	}
	l.logger.Error().Msgf("Tracer error: %s", msg)
}

// Infof logs an info message as an Zerolog Tracef.
func (l OpenTracingZerologLogger) Infof(msg string, args ...interface{}) {
	if l.logger == nil {
		return
	}
	l.logger.Trace().Msgf("Tracer: "+msg, args...)
}
