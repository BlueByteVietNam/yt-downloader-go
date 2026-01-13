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
		return utils.BadRequest(c, utils.ErrInvalidJobID, "Invalid job ID format")
	}

	// Check if job exists
	if !utils.JobExists(jobID) {
		return utils.NotFound(c, utils.ErrJobNotFound, "Job not found")
	}

	// Delete job directory
	if err := utils.DeleteJobDir(jobID); err != nil {
		return utils.InternalError(c, "Failed to delete job")
	}

	return c.JSON(models.DeleteResponse{
		Deleted: true,
	})
}
