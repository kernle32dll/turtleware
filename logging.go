package turtleware

import "strings"

type loggingOptions struct {
	logHeaders      bool
	headerWhitelist map[string]struct{}
	headerBlacklist map[string]struct{}
}

// LoggingOption represents an option for the logging parameters.
type LoggingOption func(*loggingOptions)

// Logger sets the logger to be used.
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
