module github.com/kernle32dll/turtleware/examples

go 1.24

toolchain go1.24.1

replace (
	github.com/kernle32dll/turtleware => ./..
	github.com/kernle32dll/turtleware/tenant => ./../tenant
)

require (
	github.com/kernle32dll/turtleware v0.0.0-20241006142728-f9015ed8f78d
	github.com/kernle32dll/turtleware/tenant v0.0.0-20241006142728-f9015ed8f78d
	github.com/lestrrat-go/jwx/v3 v3.0.0
	github.com/rs/zerolog v1.34.0
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/justinas/alice v1.2.0 // indirect
	github.com/kernle32dll/emissione-go v1.1.0 // indirect
	github.com/kernle32dll/keybox-go v1.2.0 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc/v3 v3.0.0-beta1 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
