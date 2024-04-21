module github.com/kernle32dll/turtleware/tenant

go 1.22

toolchain go1.22.0

replace github.com/kernle32dll/turtleware => ./..

require (
	github.com/jmoiron/sqlx v1.4.0
	github.com/justinas/alice v1.2.0
	github.com/kernle32dll/turtleware v0.0.0-20240211133627-8a740328cf04
	github.com/lestrrat-go/jwx/v2 v2.0.21
	github.com/rs/zerolog v1.33.0
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kernle32dll/emissione-go v1.1.0 // indirect
	github.com/kernle32dll/keybox-go v1.2.0 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.5 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240424034433-3c2c7870ae76 // indirect
	go.opentelemetry.io/otel v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.27.0 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
)
