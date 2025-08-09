module github.com/flunq-io/worker

go 1.23.0

toolchain go1.24.3

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.6.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.34.2
)

// Local dependencies

replace github.com/flunq-io/shared => ../shared

require (
	github.com/flunq-io/shared v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
