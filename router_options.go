package turtleware

type endpointOptions struct {
	exposedHeaders []string
	allowedHeaders []string
}

// EndpointOption represents an option for configuring an endpoint.
type EndpointOption func(*endpointOptions)

// WithExposedHeaders sets the exposed headers for the endpoint.
func WithExposedHeaders(headers ...string) EndpointOption {
	return func(opts *endpointOptions) {
		opts.exposedHeaders = headers
	}
}

// WithAllowedHeaders sets the allowed headers for the endpoint.
func WithAllowedHeaders(headers ...string) EndpointOption {
	return func(opts *endpointOptions) {
		opts.allowedHeaders = headers
	}
}
