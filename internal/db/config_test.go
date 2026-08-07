package db

import (
	"os"
	"testing"
)

func TestGetDBEngineDefault(t *testing.T) {
	os.Unsetenv("DB_ENGINE")
	engine := GetDBEngine()
	if engine != EngineSQLite {
		t.Errorf("expected default engine %q, got %q", EngineSQLite, engine)
	}
}

func TestGetDBEngineSurreal(t *testing.T) {
	os.Setenv("DB_ENGINE", "surrealdb")
	defer os.Unsetenv("DB_ENGINE")

	engine := GetDBEngine()
	if engine != EngineSurrealDB {
		t.Errorf("expected engine %q, got %q", EngineSurrealDB, engine)
	}
}

func TestGetSQLitePathDefault(t *testing.T) {
	os.Unsetenv("SQLITE_PATH")
	path := GetSQLitePath()
	if path != "./notebook.db" {
		t.Errorf("expected default path %q, got %q", "./notebook.db", path)
	}
}

func TestGetSQLitePathCustom(t *testing.T) {
	os.Setenv("SQLITE_PATH", "/tmp/custom.db")
	defer os.Unsetenv("SQLITE_PATH")

	path := GetSQLitePath()
	if path != "/tmp/custom.db" {
		t.Errorf("expected path %q, got %q", "/tmp/custom.db", path)
	}
}
