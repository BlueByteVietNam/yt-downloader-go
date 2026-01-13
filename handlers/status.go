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
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid job ID format",
		})
	}

	// Check if job exists
	if !utils.JobExists(jobID) {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "Job not found",
		})
	}

	// Read metadata
	meta, err := utils.ReadMeta(jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:  "Failed to read job metadata",
			Detail: err.Error(),
		})
	}

	// Calculate progress
	progress, detail := utils.CalculateProgress(meta)

	response := models.StatusResponse{
		Status:   meta.Status,
		Progress: progress,
		Title:    meta.Title,
		Duration: meta.Duration,
	}

	if meta.Status == "done" {
		response.Progress = 100
		response.DownloadURL = utils.GenerateSignedURL(jobID, meta.Output)
	}

	if meta.Status == "error" {
		response.Error = meta.Error
	}

	if detail != nil && meta.Status == "downloading" {
		response.Detail = detail
	}

	return c.JSON(response)
}
