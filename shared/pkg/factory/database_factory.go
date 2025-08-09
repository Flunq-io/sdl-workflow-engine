package factory

import (
	"fmt"

	redis "github.com/go-redis/redis/v8"

	"github.com/flunq-io/shared/pkg/interfaces"
	"github.com/flunq-io/shared/pkg/storage"
	postgres "github.com/flunq-io/shared/pkg/storage/postgres"
)

// DatabaseDeps encapsulates dependencies required to construct a Database implementation.
// Contributors can extend this struct with backend-specific deps via the Config.Options map
// without changing call sites.
type DatabaseDeps struct {
	Backend     string                     // e.g. "redis", "postgres"
	RedisClient redis.UniversalClient      // used when Backend == "redis"
	Logger      interfaces.Logger          // shared logger interface
	Config      *interfaces.DatabaseConfig // generic database config (tenant mode, options, etc.)
}

// NewDatabase returns a shared Database implementation based on deps.Backend.
// For unsupported backends, it returns a clear error to guide contributors.
func NewDatabase(deps DatabaseDeps) (interfaces.Database, error) {
	switch deps.Backend {
	case "", "redis":
		if deps.RedisClient == nil {
			return nil, fmt.Errorf("redis backend selected but RedisClient is nil")
		}
		if deps.Config == nil {
			deps.Config = &interfaces.DatabaseConfig{Type: "redis"}
		}
		return storage.NewRedisDatabase(deps.RedisClient, deps.Logger, deps.Config), nil
	case "postgres", "postgresql":
		return postgres.NewPostgresDatabase(deps.Logger, deps.Config), nil
	case "mongo", "mongodb":
		return nil, fmt.Errorf("mongodb backend not implemented yet; contribute an implementation in shared/pkg/storage/mongodb")
	case "dynamo", "dynamodb":
		return nil, fmt.Errorf("dynamodb backend not implemented yet; contribute an implementation in shared/pkg/storage/dynamodb")
	default:
		return nil, fmt.Errorf("unsupported DB_TYPE backend: %s", deps.Backend)
	}
}
