module github.com/flunq-io/executor

go 1.21

require (
	github.com/flunq-io/events v0.0.0-00010101000000-000000000000
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.5.0
	go.uber.org/zap v1.26.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.19.0 // indirect
)

replace github.com/flunq-io/events => ../events
replace github.com/flunq-io/shared => ../shared
