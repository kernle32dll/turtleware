module github.com/kernle32dll/turtleware/examples

go 1.23

toolchain go1.23.2

replace (
	github.com/kernle32dll/turtleware => ./..
	github.com/kernle32dll/turtleware/tenant => ./../tenant
)

require (
	github.com/kernle32dll/turtleware v0.0.0-20240725105542-317846d86b55
	github.com/kernle32dll/turtleware/tenant v0.0.0-20240725105542-317846d86b55
	github.com/lestrrat-go/jwx/v2 v2.1.1
	github.com/rs/zerolog v1.33.0
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/justinas/alice v1.2.0 // indirect
	github.com/kernle32dll/emissione-go v1.1.0 // indirect
	github.com/kernle32dll/keybox-go v1.2.0 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.6 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
)
