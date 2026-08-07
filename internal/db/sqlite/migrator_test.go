package sqlite

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteConnectionAndMigration(t *testing.T) {
	dbPath := "./test_migration.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	migrator := NewMigrator(db)
	if err := migrator.Up(); err != nil {
		t.Fatalf("failed to run sqlite migrations: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sqlite tables: %v", err)
	}

	if count == 0 {
		t.Errorf("expected tables created by migrations, got 0")
	}
}
