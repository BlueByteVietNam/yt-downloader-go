package utils

import (
	"log"
	"os"
	"path/filepath"
	"time"
	"yt-downloader-go/config"

	"github.com/robfig/cron/v3"
)

// StartCleanupScheduler starts the cleanup cron job
func StartCleanupScheduler() *cron.Cron {
	c := cron.New()

	// Run cleanup every hour
	c.AddFunc(config.CleanupInterval, func() {
		CleanupOldJobs()
	})

	c.Start()

	// Run cleanup on startup
	go CleanupOldJobs()

	log.Println("[Cleanup] Scheduler started")
	return c
}

// CleanupOldJobs removes jobs older than MaxJobAge
func CleanupOldJobs() {
	log.Println("[Cleanup] Starting cleanup...")

	// Ensure storage directory exists
	if _, err := os.Stat(config.StorageDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(config.StorageDir)
	if err != nil {
		log.Printf("[Cleanup] Error reading storage directory: %v\n", err)
		return
	}

	now := time.Now()
	deleted := 0
	processed := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		jobID := entry.Name()

		// Validate job ID format
		if !ValidateJobID(jobID) {
			// Invalid job ID, delete it
			if err := DeleteJobDir(jobID); err == nil {
				deleted++
				log.Printf("[Cleanup] Deleted invalid job: %s\n", jobID)
			}
			continue
		}

		// Check job age
		meta, err := ReadMeta(jobID)
		if err != nil {
			// Corrupted job, delete it
			if err := DeleteJobDir(jobID); err == nil {
				deleted++
				log.Printf("[Cleanup] Deleted corrupted job: %s\n", jobID)
			}
			continue
		}

		createdAt := time.UnixMilli(meta.CreatedAt)
		age := now.Sub(createdAt)

		if age > config.MaxJobAge {
			if err := DeleteJobDir(jobID); err == nil {
				deleted++
				log.Printf("[Cleanup] Deleted old job: %s (age: %v)\n", jobID, age.Round(time.Minute))
			}
		}

		processed++
		if processed >= config.CleanupBatchSize {
			break
		}
	}

	log.Printf("[Cleanup] Finished. Deleted %d jobs\n", deleted)
}

// CleanupTempFiles removes temporary files from a job directory
func CleanupTempFiles(jobID string) error {
	jobDir := GetJobDir(jobID)

	patterns := []string{
		"*.tmp",
		"video.*",
		"audio.*",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(jobDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			// Don't delete the output file
			if filepath.Base(match) != "output" {
				os.Remove(match)
			}
		}
	}

	return nil
}
