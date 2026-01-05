package handlers

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"yt-downloader-go/models"
	"yt-downloader-go/utils"

	"github.com/gofiber/fiber/v2"
)

// HandleFiles handles GET /files/:id/:filename
func HandleFiles(c *fiber.Ctx) error {
	jobID := c.Params("id")
	filename := c.Params("filename")

	// Validate job ID
	if !utils.ValidateJobID(jobID) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid job ID format",
		})
	}

	// Validate filename (prevent path traversal)
	if !utils.ValidateFilename(filename) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid filename",
		})
	}

	// Check if job exists
	if !utils.JobExists(jobID) {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "Job not found",
		})
	}

	// Read metadata to get actual output filename
	meta, err := utils.ReadMeta(jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to read job metadata",
		})
	}

	// Check if job is done
	if meta.Status != "done" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Job is not completed yet",
		})
	}

	// Build file path
	filePath := filepath.Join(utils.GetJobDir(jobID), filename)

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "File not found",
		})
	}

	// Get content type
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	contentType := utils.ContentTypeFromExt(ext)

	// Generate download filename
	downloadFilename := utils.GenerateOutputFilename(meta)

	// RFC 5987 encoding for non-ASCII characters
	encodedFilename := url.PathEscape(downloadFilename)

	// Set headers
	c.Set("Content-Type", contentType)
	c.Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, downloadFilename, encodedFilename))

	// Stream file
	return c.SendFile(filePath)
}
