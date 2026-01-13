package handlers

import (
	"yt-downloader-go/models"
	"yt-downloader-go/utils"

	"github.com/gofiber/fiber/v2"
)

// HandleStatus handles GET /api/status/:id
func HandleStatus(c *fiber.Ctx) error {
	jobID := c.Params("id")

	// Validate job ID
	if !utils.ValidateJobID(jobID) {
		return utils.BadRequest(c, utils.ErrInvalidJobID, "Invalid job ID format")
	}

	// Check if job exists
	if !utils.JobExists(jobID) {
		return utils.NotFound(c, utils.ErrJobNotFound, "Job not found")
	}

	// Read metadata
	meta, err := utils.ReadMeta(jobID)
	if err != nil {
		return utils.InternalError(c, "Failed to read job metadata")
	}

	// Calculate progress
	progress, detail := utils.CalculateProgress(meta)

	response := models.StatusResponse{
		Status:   meta.Status,
		Progress: progress,
		Title:    meta.Title,
		Duration: meta.Duration,
	}

	// Set downloadUrl when completed
	if meta.Status == models.StatusCompleted {
		response.Progress = 100
		if meta.Output != "" {
			// Merged file available - use static file URL
			response.DownloadURL = utils.GenerateSignedURL(jobID, meta.Output)
		} else if meta.StreamOnly {
			// Stream only - use stream URL
			response.DownloadURL = utils.GenerateStreamURL(jobID)
		}
	}

	// Set jobError when error
	if meta.Status == models.StatusError {
		response.JobError = meta.Error
	}

	// Set detail only when pending
	if meta.Status == models.StatusPending && detail != nil {
		response.Detail = detail
	}

	return c.JSON(response)
}
