package db

import (
	"os"
	"strings"
)

// EngineType represents the backend database engine type
type EngineType string

const (
	EngineSQLite    EngineType = "sqlite"
	EngineSurrealDB EngineType = "surrealdb"
)

// GetDBEngine returns the configured database engine, defaulting to sqlite
func GetDBEngine() EngineType {
	engine := strings.ToLower(strings.TrimSpace(os.Getenv("DB_ENGINE")))
	switch engine {
	case "surreal", "surrealdb":
		return EngineSurrealDB
	case "sqlite":
		return EngineSQLite
	default:
		return EngineSQLite
	}
}

// GetSQLitePath returns the configured SQLite database file path
func GetSQLitePath() string {
	if path := os.Getenv("SQLITE_PATH"); path != "" {
		return path
	}
	return "./notebook.db"
}
