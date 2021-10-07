package turtleware

import (
	"github.com/sirupsen/logrus"
)

// OpenTracingLogrusLogger is an implementation of the openTracing Logger interface
// that delegates to a given Logrus logger. If the logger is nil, logrus.StandardLogger()
// is used.
//
type OpenTracingLogrusLogger struct {
	logger *logrus.Logger
}

// Error logs an error message as an Logrus Errorf.
func (l *OpenTracingLogrusLogger) Error(msg string) {
	logger := l.logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	logger.Errorf("Tracer error: %s", msg)
}

// Infof logs an info message as an Logrus Tracef.
func (l *OpenTracingLogrusLogger) Infof(msg string, args ...interface{}) {
	logger := l.logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	logger.Tracef("Tracer: "+msg, args...)
}
