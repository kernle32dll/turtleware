package turtleware

type loggingOptions struct {
	logHeaders      bool
	headerWhitelist map[string]string
	headerBlacklist map[string]string
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

// LoggerHeaderWhitelist sets a whitelist of headers to allow.
// Automatically replaces the blacklist if used.
// The default is not set, which means "allow all".
func LoggerHeaderWhitelist(headerWhitelist map[string]string) LoggingOption {
	return func(c *loggingOptions) {
		c.headerWhitelist = headerWhitelist
		c.headerBlacklist = nil
	}
}

// LoggerHeaderWhitelist sets a whitelist of headers to allow.
// Automatically replaces the blacklist if used.
// The default is not set, which means "allow all".
func LoggerHeaderBlacklist(headerBlacklist map[string]string) LoggingOption {
	return func(c *loggingOptions) {
		c.headerWhitelist = nil
		c.headerBlacklist = headerBlacklist
	}
}
