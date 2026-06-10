package main

import (
	"context"
	"fmt"
	"go-notebook/internal/db"
	"go-notebook/internal/worker"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	// Parse arguments manually
	if len(os.Args) > 1 {
		handleDirectCommand(os.Args[1:])
		return
	}

	// Default: Run as a background worker daemon
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Connect to database
	if err := db.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database connection: %v", err)
	}
	defer db.Close(ctx)

	log.Println("[Worker] Starting background worker...")
	worker.Start(ctx)
	log.Println("[Worker] Background worker stopped.")
}

func handleDirectCommand(args []string) {
	cmdName := args[0]
	if cmdName == "help" || cmdName == "--help" || cmdName == "-h" {
		printHelp()
		return
	}

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database connection: %v", err)
	}
	defer db.Close(ctx)

	switch cmdName {
	case "worker":
		// Explicit daemon start
		log.Println("[Worker] Starting background worker daemon...")
		worker.Start(ctx)
	default:
		log.Fatalf("Unknown command: %s. Run with 'help' for available commands.", cmdName)
	}
}

func printHelp() {
	fmt.Println("Open Notebook Background Worker CLI")
	fmt.Println("Usage:")
	fmt.Println("  worker          Start the background worker polling loop (default)")
	fmt.Println("  help            Show this help info")
}
