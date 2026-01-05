package handlers

import (
	"time"
	"yt-downloader-go/models"

	"github.com/gofiber/fiber/v2"
)

// HandleHealth handles GET /health
func HandleHealth(c *fiber.Ctx) error {
	return c.JSON(models.HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UnixMilli(),
	})
}
