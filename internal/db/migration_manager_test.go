package db

import (
	"strings"
	"testing"
)

func TestReadMigrationFile(t *testing.T) {
	sql, err := ReadMigrationFile("1.surrealql")
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	if sql == "" {
		t.Error("expected sql content to be non-empty")
	}

	// Verify comments are stripped out
	if strings.Contains(sql, "--") {
		t.Errorf("expected comments starting with -- to be cleaned out, got: %s", sql)
	}
}
