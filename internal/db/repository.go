package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// DB is the global SurrealDB client instance
var DB *surrealdb.DB

// GetDatabaseURL returns the connection URL with backward compatibility
func GetDatabaseURL() string {
	if url := os.Getenv("SURREAL_URL"); url != "" {
		return url
	}

	address := os.Getenv("SURREAL_ADDRESS")
	if address == "" {
		address = "localhost"
	}
	port := os.Getenv("SURREAL_PORT")
	if port == "" {
		port = "8000"
	}

	// Default to ws connection for SurrealDB Go SDK
	return fmt.Sprintf("ws://%s:%s", address, port)
}

// GetDatabasePassword returns the database password supporting legacy key names
func GetDatabasePassword() string {
	if pass := os.Getenv("SURREAL_PASSWORD"); pass != "" {
		return pass
	}
	return os.Getenv("SURREAL_PASS")
}

// Init initializes the global SurrealDB client connection
func Init(ctx context.Context) error {
	url := GetDatabaseURL()
	user := os.Getenv("SURREAL_USER")
	pass := GetDatabasePassword()
	ns := os.Getenv("SURREAL_NAMESPACE")
	dbName := os.Getenv("SURREAL_DATABASE")

	if ns == "" {
		ns = "open_notebook"
	}
	if dbName == "" {
		dbName = "open_notebook"
	}

	log.Printf("[DB] Connecting to SurrealDB at %s...", url)

	client, err := surrealdb.FromEndpointURLString(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to create surrealdb client: %w", err)
	}

	if err := client.Use(ctx, ns, dbName); err != nil {
		client.Close(ctx)
		return fmt.Errorf("failed to select namespace/database: %w", err)
	}

	if user != "" || pass != "" {
		_, err = client.SignIn(ctx, surrealdb.Auth{
			Username: user,
			Password: pass,
		})
		if err != nil {
			client.Close(ctx)
			return fmt.Errorf("failed to sign in: %w", err)
		}
	}

	DB = client
	log.Println("[DB] Successfully connected and authenticated with SurrealDB")
	return nil
}

// Close closes the global SurrealDB connection
func Close(ctx context.Context) {
	if DB != nil {
		DB.Close(ctx)
		log.Println("[DB] Closed SurrealDB connection")
	}
}

// RepoQuery executes a raw SurrealQL query and unmarshals the response into T
func RepoQuery[T any](ctx context.Context, queryStr string, vars map[string]any) (*T, error) {
	if DB == nil {
		return nil, errors.New("database connection not initialized")
	}

	result, err := surrealdb.Query[T](ctx, DB, queryStr, vars)
	if err != nil {
		// Log debug for transaction conflicts to match log noise reduction in Python
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "transaction") || strings.Contains(errStr, "conflict") {
			log.Printf("[DB] Transaction conflict (retriable): %v", err)
		} else {
			log.Printf("[DB] Query error: %v", err)
		}
		return nil, err
	}

	if result == nil || len(*result) == 0 {
		return nil, fmt.Errorf("empty query results")
	}

	qr := (*result)[0]
	if qr.Error != nil {
		return nil, qr.Error
	}

	return &qr.Result, nil
}

// RepoCreate creates a new record in the specified table
func RepoCreate[T any](ctx context.Context, table string, data map[string]any) (*T, error) {
	if DB == nil {
		return nil, errors.New("database connection not initialized")
	}

	// Ensure timestamps are added and ID is removed if present (to let SurrealDB auto-generate it)
	delete(data, "id")
	now := time.Now().UTC()
	data["created"] = now
	data["updated"] = now

	// We use Query to insert to ensure generic compatibility and exact auto-ID generation
	query := fmt.Sprintf("CREATE %s CONTENT $data;", table)
	results, err := RepoQuery[[]T](ctx, query, map[string]any{"data": data})
	if err != nil {
		return nil, err
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("failed to create record: empty result returned")
	}

	// CREATE table CONTENT returns an array of created records. Return the first one.
	return &(*results)[0], nil
}

// RepoUpdate updates an existing record by ID
func RepoUpdate[T any](ctx context.Context, table string, id string, data map[string]any) (*T, error) {
	if DB == nil {
		return nil, errors.New("database connection not initialized")
	}

	// Format record ID
	recordID := id
	if !strings.Contains(id, ":") {
		recordID = fmt.Sprintf("%s:%s", table, id)
	}

	delete(data, "id")
	data["updated"] = time.Now().UTC()

	// Perform MERGE update
	query := fmt.Sprintf("UPDATE %s MERGE $data;", recordID)
	results, err := RepoQuery[[]T](ctx, query, map[string]any{"data": data})
	if err != nil {
		return nil, fmt.Errorf("failed to update record: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("record not found or update failed")
	}

	return &(*results)[0], nil
}

// RepoDelete deletes a record by record ID
func RepoDelete(ctx context.Context, recordID string) error {
	if DB == nil {
		return errors.New("database connection not initialized")
	}

	var parsedID models.RecordID
	parsed, err := models.ParseRecordID(recordID)
	if err == nil {
		parsedID = *parsed
	} else {
		// Try to parse with table prefix if simple string given, but we expect table:id format
		parts := strings.SplitN(recordID, ":", 2)
		if len(parts) == 2 {
			parsedID = models.NewRecordID(parts[0], parts[1])
		} else {
			return fmt.Errorf("invalid record ID format (expected table:id): %s", recordID)
		}
	}

	_, err = surrealdb.Delete[any](ctx, DB, parsedID)
	if err != nil {
		log.Printf("[DB] Delete error: %v", err)
		return err
	}

	return nil
}

// RepoRelate creates a graph relationship between two records
func RepoRelate(ctx context.Context, source, relationship, target string, data map[string]any) error {
	if data == nil {
		data = make(map[string]any)
	}

	// Construct RELATE query
	query := fmt.Sprintf("RELATE %s->%s->%s CONTENT $data;", source, relationship, target)
	_, err := RepoQuery[any](ctx, query, map[string]any{"data": data})
	return err
}

// EnsureRecordIDString creates a full table:id record ID string from string components
func EnsureRecordIDString(table, id string) string {
	if strings.Contains(id, ":") {
		return id
	}
	return fmt.Sprintf("%s:%s", table, id)
}

// EnsureRecordID creates a *models.RecordID from string components
func EnsureRecordID(table, id string) *models.RecordID {
	if id == "" {
		return nil
	}
	recordID := id
	if !strings.Contains(id, ":") {
		recordID = fmt.Sprintf("%s:%s", table, id)
	}
	parsed, err := models.ParseRecordID(recordID)
	if err != nil {
		parts := strings.SplitN(recordID, ":", 2)
		rec := models.NewRecordID(parts[0], parts[1])
		return &rec
	}
	return parsed
}

// RepoUpsert upserts a record by ID. If it exists, updates it. If not, creates it.
func RepoUpsert[T any](ctx context.Context, table string, id string, data map[string]any, overwrite bool) (*T, error) {
	if DB == nil {
		return nil, errors.New("database connection not initialized")
	}

	// Format record ID
	recordID := id
	if !strings.Contains(id, ":") {
		recordID = fmt.Sprintf("%s:%s", table, id)
	}

	delete(data, "id")
	now := time.Now().UTC()
	if _, exists := data["created"]; !exists {
		data["created"] = now
	}
	data["updated"] = now

	// Perform UPSERT
	query := fmt.Sprintf("UPSERT %s CONTENT $data;", recordID)
	results, err := RepoQuery[[]T](ctx, query, map[string]any{"data": data})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert record: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("upsert failed to return result")
	}

	return &(*results)[0], nil
}
