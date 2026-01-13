package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"yt-downloader-go/config"
	"yt-downloader-go/handlers"
	"yt-downloader-go/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Create storage directory
	if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	// Start cleanup scheduler
	cleanupCron := utils.StartCleanupScheduler()
	defer cleanupCron.Stop()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:       "YouTube Downloader Go",
		ServerHeader:  "yt-downloader-go",
		CaseSensitive: true,
		StrictRouting: false,
		// Disable body limit for file streaming
		BodyLimit: 0,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Accept",
	}))

	// API routes
	api := app.Group("/api")
	api.Post("/download", handlers.HandleDownload)
	api.Get("/status/:id", handlers.HandleStatus)
	api.Delete("/jobs/:id", handlers.HandleDeleteJob)

	// File serving
	app.Get("/files/:id/:filename", handlers.HandleFiles)

	// Stream serving (FFmpeg pipe)
	app.Get("/stream/:id", handlers.HandleStream)

	// Health check
	app.Get("/health", handlers.HandleHealth)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Printf("Error shutting down: %v\n", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("Starting server on http://localhost%s\n", addr)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
