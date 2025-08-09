module github.com/flunq-io/timer

go 1.23.0

toolchain go1.24.3

require (
	github.com/go-redis/redis/v8 v8.11.5
	go.uber.org/zap v1.27.0
)

replace github.com/flunq-io/shared => ../shared

require github.com/flunq-io/shared v0.0.0-00010101000000-000000000000

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)
