![test](https://github.com/kernle32dll/turtleware/workflows/test/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/kernle32dll/turtleware.svg)](https://pkg.go.dev/github.com/kernle32dll/turtleware)
[![Go Report Card](https://goreportcard.com/badge/github.com/kernle32dll/turtleware)](https://goreportcard.com/report/github.com/kernle32dll/turtleware)
[![codecov](https://codecov.io/gh/kernle32dll/turtleware/branch/master/graph/badge.svg)](https://codecov.io/gh/kernle32dll/turtleware)

# turtleware

turtleware is an opinionated framework for creating REST services. It provides pluggable middlewares and some utility
methods to simplify life. Its uses JWT bearer authentication, and relies heavily on caching.

The framework is hardwired to use both [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx) for JWT parsing,
and [zerolog](https://github.com/rs/zerolog) for logging. Extensive use is made of propagating loggers via Go
context.

Download:

```
go get github.com/kernle32dll/turtleware
```

Detailed documentation can be found on [pkg.go.dev](https://pkg.go.dev/github.com/kernle32dll/turtleware).

## State of the project

turtleware is actively used in productive projects by the author.

Still, this project is still pretty much work-in-progress. Bugs happen, and breaking-changes might occur at **any**
time. Also, only the most recent Go version is supported at any time for now. Code coverage is low, and documentation
slim, so be warned.

## Getting started

turtleware provides three distinct functionalities:

1. A set of middlewares, which can be chained individually (e.g. auth)
2. Composition methods for chaining these middlewares together in a meaningful way (e.g. a GET endpoint)
3. Optional multi tenancy

For a complete example, look at the [main.go in the examples folder](examples/main.go).