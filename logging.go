package turtleware

import (
	"github.com/sirupsen/logrus"

	"strings"
)

type loggingOptions struct {
	logHeaders      bool
	headerWhitelist map[string]struct{}
	headerBlacklist map[string]struct{}
}

// LoggingOption represents an option for the logging parameters.
type LoggingOption func(*loggingOptions)

// LogHeaders sets whether or not headers hould be logged.
// The default is false.
func LogHeaders(logHeaders bool) LoggingOption {
	return func(c *loggingOptions) {
		c.logHeaders = logHeaders
	}
}

// LogHeaderWhitelist sets a whitelist of headers to allow.
// Automatically replaces the blacklist if used.
// The default is not set, which means "allow all".
func LogHeaderWhitelist(headerWhitelist ...string) LoggingOption {
	return func(c *loggingOptions) {
		c.headerWhitelist = make(map[string]struct{}, len(headerWhitelist))
		c.headerBlacklist = nil

		for _, header := range headerWhitelist {
			c.headerWhitelist[strings.ToLower(header)] = struct{}{}
		}
	}
}

// LogHeaderBlacklist sets a blacklist of headers to disallow.
// Automatically replaces the whitelist if used.
// The default is not set, which means "allow all".
func LogHeaderBlacklist(headerBlacklist ...string) LoggingOption {
	return func(c *loggingOptions) {
		c.headerWhitelist = nil
		c.headerBlacklist = make(map[string]struct{}, len(headerBlacklist))

		for _, header := range headerBlacklist {
			c.headerBlacklist[strings.ToLower(header)] = struct{}{}
		}
	}
}

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
