package worker

import (
	"context"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"log"
	"time"
)

// Start starts the command polling loop
func Start(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	log.Println("[Worker] Daemon started, polling for commands...")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Worker] Stopping background worker daemon...")
			return
		case <-ticker.C:
			pollAndProcessJobs(ctx)
		}
	}
}

func pollAndProcessJobs(ctx context.Context) {
	// Query pending commands
	results, err := db.RepoQuery[[]domain.CommandJob](ctx, "SELECT * FROM command WHERE status = 'pending' ORDER BY created ASC;", nil)
	if err != nil {
		return
	}
	if results == nil || len(*results) == 0 {
		return
	}

	for _, job := range *results {
		jobID := job.ID.String()
		nowStr := time.Now().UTC().Format(time.RFC3339)

		// Atomic claim: set status to running only if it is still pending
		claimResult, err := db.RepoQuery[[]domain.CommandJob](ctx, "UPDATE ONLY $id SET status = 'running', updated = $now WHERE status = 'pending' RETURN AFTER;", map[string]any{
			"id":  jobID,
			"now": nowStr,
		})
		if err != nil || claimResult == nil || len(*claimResult) == 0 {
			// Job was claimed by another thread or worker, skip
			continue
		}

		// Execute job
		claimedJob := &(*claimResult)[0]
		res, execErr := ExecuteJob(ctx, claimedJob)

		nowStr = time.Now().UTC().Format(time.RFC3339)
		if execErr != nil {
			log.Printf("[Worker] Job %s failed: %v", jobID, execErr)

			// Retry logic
			maxAttempts := 3
			if claimedJob.Command == "process_source" {
				maxAttempts = 5
			} else if claimedJob.Command == "generate_podcast" {
				maxAttempts = 1
			}

			if claimedJob.RetryAttempts+1 < maxAttempts {
				log.Printf("[Worker] Retrying job %s (attempt %d/%d)...", jobID, claimedJob.RetryAttempts+1, maxAttempts)
				_, _ = db.RepoQuery[any](ctx, "UPDATE ONLY $id SET status = 'pending', retry_attempts = $attempts, error_message = $err, updated = $now;", map[string]any{
					"id":       jobID,
					"attempts": claimedJob.RetryAttempts + 1,
					"err":      execErr.Error(),
					"now":      nowStr,
				})
			} else {
				log.Printf("[Worker] Job %s reached max attempts (%d). Marking as failed.", jobID, maxAttempts)
				_, _ = db.RepoQuery[any](ctx, "UPDATE ONLY $id SET status = 'failed', error_message = $err, updated = $now;", map[string]any{
					"id":  jobID,
					"err": execErr.Error(),
					"now": nowStr,
				})
			}
		} else {
			log.Printf("[Worker] Job %s completed successfully.", jobID)
			_, _ = db.RepoQuery[any](ctx, "UPDATE ONLY $id SET status = 'success', result = $res, updated = $now;", map[string]any{
				"id":  jobID,
				"res": res,
				"now": nowStr,
			})
		}
	}
}
