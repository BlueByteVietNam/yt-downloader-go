package handlers

import (
	"context"
	"log"
	"time"
	"yt-downloader-go/config"
	"yt-downloader-go/models"
	"yt-downloader-go/services"
	"yt-downloader-go/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/jaevor/go-nanoid"
)

var generateID func() string

func init() {
	// Initialize nanoid generator
	var err error
	generateID, err = nanoid.Standard(config.JobIDLength)
	if err != nil {
		panic(err)
	}
}

// HandleDownload handles POST /api/download
func HandleDownload(c *fiber.Ctx) error {
	var req models.DownloadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// Validate request
	if err := utils.ValidateDownloadRequest(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	// Extract video ID
	videoID, err := utils.ExtractVideoID(req.URL)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: err.Error(),
		})
	}

	// Fetch video metadata
	extractData, err := services.Extract(videoID)
	if err != nil {
		log.Printf("[Download] Extract API error: %v\n", err)
		return c.Status(fiber.StatusBadGateway).JSON(models.ErrorResponse{
			Error:  "Failed to fetch video metadata",
			Detail: err.Error(),
		})
	}

	// Set default values
	osType := req.OS
	if osType == "" {
		osType = "windows"
	}
	bitrate := req.Audio.Bitrate
	if bitrate == "" {
		bitrate = "192k"
	}

	// Select streams
	var videoSelection *models.VideoSelectionResult
	var audioStream *models.Stream

	if req.Output.Type == "video" {
		videoSelection = services.SelectVideo(extractData, req.Output.Quality, osType)
		if videoSelection.Stream == nil {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Error: "No compatible video stream found",
			})
		}
		audioStream = services.SelectAudio(extractData, req.Audio.TrackID, osType)
		if audioStream == nil {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Error: "No compatible audio stream found",
			})
		}
	} else {
		audioStream = services.SelectAudio(extractData, req.Audio.TrackID, osType)
		if audioStream == nil {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Error: "No compatible audio stream found",
			})
		}
	}

	// Generate job ID
	jobID := generateID()

	// Create job directory
	if err := utils.CreateJobDir(jobID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to create job directory",
		})
	}

	// Prepare metadata
	meta := &models.Meta{
		ID:         jobID,
		Status:     "downloading",
		CreatedAt:  time.Now().UnixMilli(),
		VideoID:    videoID,
		Title:      extractData.Title,
		Duration:   extractData.Duration,
		OutputType: req.Output.Type,
		Format:     req.Output.Format,
		Bitrate:    bitrate,
		Trim:       req.Trim,
		Files:      models.FilesInfo{},
	}

	// Set file info
	if req.Output.Type == "video" {
		videoExt := services.GetExtension(videoSelection.Stream)
		audioExt := services.GetExtension(audioStream)
		meta.Quality = videoSelection.SelectedQuality
		meta.Files.Video = &models.FileInfo{
			Name: "video." + videoExt,
			Size: videoSelection.Stream.ContentLength,
		}
		meta.Files.Audio = &models.FileInfo{
			Name: "audio." + audioExt,
			Size: audioStream.ContentLength,
		}
	} else {
		audioExt := services.GetExtension(audioStream)
		meta.Files.Audio = &models.FileInfo{
			Name: "audio." + audioExt,
			Size: audioStream.ContentLength,
		}
	}

	// Save metadata
	if err := utils.WriteMeta(jobID, meta); err != nil {
		utils.DeleteJobDir(jobID)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to save job metadata",
		})
	}

	// Start background processing
	go processJob(jobID, meta, videoSelection, audioStream, req.Output.Format, bitrate)

	// Build response
	response := models.DownloadResponse{
		ID:       jobID,
		Title:    extractData.Title,
		Duration: extractData.Duration,
	}

	if req.Output.Type == "video" && videoSelection != nil {
		response.RequestedQuality = req.Output.Quality
		response.SelectedQuality = videoSelection.SelectedQuality
		response.QualityChanged = videoSelection.QualityChanged
		response.QualityChangeReason = videoSelection.QualityChangeReason
		response.NeedsReencode = videoSelection.NeedsReencode
	}

	return c.JSON(response)
}

// processJob handles the background download and processing
func processJob(jobID string, meta *models.Meta, videoSelection *models.VideoSelectionResult, audioStream *models.Stream, format string, bitrate string) {
	// Timeout: 30 minutes max per job to prevent zombie goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	jobDir := utils.GetJobDir(jobID)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Job %s] Panic: %v\n", jobID, r)
			utils.UpdateMetaError(jobID, "Internal error")
		}
	}()

	log.Printf("[Job %s] Starting download...\n", jobID)

	// Download files
	if meta.OutputType == "video" {
		// Download video and audio in parallel
		errChan := make(chan error, 2)

		go func() {
			videoPath := jobDir + "/" + meta.Files.Video.Name
			errChan <- services.Download(ctx, videoSelection.Stream.URL, videoPath, videoSelection.Stream.ContentLength)
		}()

		go func() {
			audioPath := jobDir + "/" + meta.Files.Audio.Name
			errChan <- services.Download(ctx, audioStream.URL, audioPath, audioStream.ContentLength)
		}()

		// Wait for both downloads
		for i := 0; i < 2; i++ {
			if err := <-errChan; err != nil {
				log.Printf("[Job %s] Download error: %v\n", jobID, err)
				utils.UpdateMetaError(jobID, "Download failed: "+err.Error())
				return
			}
		}
	} else {
		// Download audio only
		audioPath := jobDir + "/" + meta.Files.Audio.Name
		if err := services.Download(ctx, audioStream.URL, audioPath, audioStream.ContentLength); err != nil {
			log.Printf("[Job %s] Download error: %v\n", jobID, err)
			utils.UpdateMetaError(jobID, "Download failed: "+err.Error())
			return
		}
	}

	log.Printf("[Job %s] Download complete, processing...\n", jobID)
	utils.UpdateMetaStatus(jobID, "processing")

	// Process with FFmpeg
	var outputFile string
	var err error

	if meta.OutputType == "video" {
		// Merge video and audio
		outputFile, err = services.FFmpegMerge(jobDir, format, meta.Files.Video.Name, meta.Files.Audio.Name)
		if err != nil {
			log.Printf("[Job %s] FFmpeg merge error: %v\n", jobID, err)
			utils.UpdateMetaError(jobID, "Processing failed: "+err.Error())
			return
		}

		// Trim if requested
		if meta.Trim != nil {
			outputFile, err = services.FFmpegTrim(jobDir, format, meta.Trim, bitrate)
			if err != nil {
				log.Printf("[Job %s] FFmpeg trim error: %v\n", jobID, err)
				utils.UpdateMetaError(jobID, "Trim failed: "+err.Error())
				return
			}
		}
	} else {
		// Convert audio
		outputFile, err = services.FFmpegConvertAudio(jobDir, format, bitrate, meta.Files.Audio.Name)
		if err != nil {
			log.Printf("[Job %s] FFmpeg convert error: %v\n", jobID, err)
			utils.UpdateMetaError(jobID, "Conversion failed: "+err.Error())
			return
		}

		// Trim if requested
		if meta.Trim != nil {
			outputFile, err = services.FFmpegTrimAudio(jobDir, format, meta.Trim, bitrate)
			if err != nil {
				log.Printf("[Job %s] FFmpeg trim error: %v\n", jobID, err)
				utils.UpdateMetaError(jobID, "Trim failed: "+err.Error())
				return
			}
		}
	}

	// Cleanup temp files
	utils.CleanupTempFiles(jobID)

	// Update meta with output
	utils.UpdateMetaOutput(jobID, outputFile)

	log.Printf("[Job %s] Completed: %s\n", jobID, outputFile)
}
