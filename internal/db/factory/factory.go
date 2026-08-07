package factory

import (
	"context"
	"fmt"
	"log"

	"go-notebook/internal/db"
	"go-notebook/internal/db/repository"
	"go-notebook/internal/db/sqlite"
	"go-notebook/internal/db/surrealdb"
)

var GlobalFactory repository.RepositoryFactory

// InitFactory initializes the active database repository factory based on DB_ENGINE
func InitFactory(ctx context.Context) (repository.RepositoryFactory, error) {
	engine := db.GetDBEngine()
	log.Printf("[DB] Initializing database engine: %s", engine)

	switch engine {
	case db.EngineSQLite:
		path := db.GetSQLitePath()
		factory, err := sqlite.NewSQLiteFactory(path)
		if err != nil {
			return nil, fmt.Errorf("sqlite init error: %w", err)
		}
		GlobalFactory = factory
		return factory, nil
	case db.EngineSurrealDB:
		factory, err := surrealdb.NewSurrealFactory(ctx)
		if err != nil {
			return nil, fmt.Errorf("surrealdb init error: %w", err)
		}
		GlobalFactory = factory
		return factory, nil
	default:
		return nil, fmt.Errorf("unsupported database engine: %s", engine)
	}
}
