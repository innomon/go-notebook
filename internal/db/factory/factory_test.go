package factory

import (
	"context"
	"os"
	"testing"
)

func TestInitFactorySQLite(t *testing.T) {
	dbPath := "./test_factory.db"
	defer os.Remove(dbPath)

	os.Setenv("DB_ENGINE", "sqlite")
	os.Setenv("SQLITE_PATH", dbPath)
	defer os.Unsetenv("DB_ENGINE")
	defer os.Unsetenv("SQLITE_PATH")

	factory, err := InitFactory(context.Background())
	if err != nil {
		t.Fatalf("failed to initialize sqlite factory: %v", err)
	}
	defer factory.Close(context.Background())

	if factory.Notes() == nil {
		t.Errorf("expected non-nil Notes repository")
	}
}
