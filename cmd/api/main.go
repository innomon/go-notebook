package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-notebook/frontend"
	"go-notebook/internal/api/router"
	"go-notebook/internal/db"
	"go-notebook/internal/utils"
	"go-notebook/internal/worker"

	"github.com/joho/godotenv"
)


func main() {
	// 1. Load environment variables (.env files)
	if err := godotenv.Load(); err != nil {
		log.Println("[API] Info: No .env file found, using system environment variables")
	}

	// 2. Setup Context and database connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform security checks
	encKey := utils.GetSecretFromEnv("OPEN_NOTEBOOK_ENCRYPTION_KEY")
	if encKey == "" {
		log.Println("[API] Warning: OPEN_NOTEBOOK_ENCRYPTION_KEY not set. API key encryption will fail until this is configured.")
	}

	// Initialize SurrealDB
	if err := db.Init(ctx); err != nil {
		log.Fatalf("[API] Critical: Database connection failed: %v", err)
	}
	defer db.Close(context.Background())

	// 3. Execute DB Migrations
	migrationCtx, migrationCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer migrationCancel()
	if err := db.RunMigrationUp(migrationCtx); err != nil {
		log.Fatalf("[API] Critical: Database migration failed: %v", err)
	}

	// 4. Extract frontend sub-filesystem
	subFS, err := fs.Sub(frontend.Assets, "out")
	if err != nil {
		log.Fatalf("[API] Critical: Failed to load embedded frontend: %v", err)
	}

	// 5. Setup ServeMux Router
	r := router.NewRouter(subFS)

	// 6. Start the Background Worker Daemon goroutine
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	log.Println("[Worker] Starting background worker goroutine...")
	go worker.Start(workerCtx)

	// Get PORT from env or fallback to original FastAPI port
	port := os.Getenv("PORT")
	if port == "" {
		port = "5055"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// 7. Graceful shutdown signaling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[API] Server listening on port %s...", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[API] Server error: %v", err)
		}
	}()

	<-stop
	log.Println("[API] Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[API] Server shutdown error: %v", err)
	}
	log.Println("[API] Server stopped")
}
