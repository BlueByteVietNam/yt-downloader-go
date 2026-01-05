package handlers

import (
	"yt-downloader-go/models"
	"yt-downloader-go/utils"

	"github.com/gofiber/fiber/v2"
)

// HandleDeleteJob handles DELETE /api/jobs/:id
func HandleDeleteJob(c *fiber.Ctx) error {
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

	// Delete job directory
	if err := utils.DeleteJobDir(jobID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:  "Failed to delete job",
			Detail: err.Error(),
		})
	}

	return c.JSON(models.DeleteResponse{
		Success: true,
	})
}
