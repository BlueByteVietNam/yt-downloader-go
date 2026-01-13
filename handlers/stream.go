package handlers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"yt-downloader-go/config"
	"yt-downloader-go/models"
	"yt-downloader-go/utils"

	"github.com/gofiber/fiber/v2"
)

// HandleStream handles GET /stream/:id
// Streams video/audio using FFmpeg pipe (realtime remux/convert)
func HandleStream(c *fiber.Ctx) error {
	jobID := c.Params("id")
	token := c.Query("token")
	expiresStr := c.Query("expires")

	// Validate job ID
	if !utils.ValidateJobID(jobID) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid job ID format",
		})
	}

	// Validate signed URL
	if token == "" || expiresStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Error: "Missing token or expires parameter",
		})
	}

	expires, err := utils.ParseExpires(expiresStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Invalid expires parameter",
		})
	}

	if !utils.ValidateStreamURL(jobID, token, expires) {
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
			Error: "Invalid or expired stream link",
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
			Error: "Failed to read job metadata",
		})
	}

	// Check if job is ready for streaming
	if meta.Status != "ready" && meta.Status != "done" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error: "Job is not ready for streaming",
		})
	}

	// If already merged, redirect to file download
	if meta.Status == "done" && meta.Output != "" && !meta.StreamOnly {
		downloadURL := utils.GenerateSignedURL(jobID, meta.Output)
		return c.Redirect(downloadURL, fiber.StatusTemporaryRedirect)
	}

	// Stream based on output type
	if meta.OutputType == "video" {
		return streamVideo(c, meta)
	}
	return streamAudio(c, meta)
}

// streamVideo streams merged video+audio using FFmpeg remux
func streamVideo(c *fiber.Ctx, meta *models.Meta) error {
	jobDir := utils.GetJobDir(meta.ID)
	videoPath := filepath.Join(jobDir, meta.Files.Video.Name)
	audioPath := filepath.Join(jobDir, meta.Files.Audio.Name)

	// Check files exist
	if _, err := os.Stat(videoPath); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "Video file not found",
		})
	}
	if _, err := os.Stat(audioPath); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "Audio file not found",
		})
	}

	// Determine output format and content type
	format := meta.Format
	contentType := utils.ContentTypeFromExt(format)

	// Generate filename
	filename := utils.GenerateOutputFilename(meta)
	encodedFilename := url.PathEscape(filename)

	// Set response headers
	c.Set("Content-Type", contentType)
	c.Set("Transfer-Encoding", "chunked")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename))
	c.Set("Cache-Control", "no-cache")

	// Build FFmpeg command for remuxing (no re-encoding, very light CPU)
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-f", getFFmpegFormat(format),
	}

	// Add movflags for streamable MP4
	if format == "mp4" {
		args = append(args, "-movflags", "frag_keyframe+empty_moov+faststart")
	}

	args = append(args, "pipe:1")

	log.Printf("[Stream %s] Starting video remux: %s + %s -> %s\n", meta.ID, meta.Files.Video.Name, meta.Files.Audio.Name, format)

	return runFFmpegStream(c, args, meta.ID)
}

// streamAudio streams audio, with transcoding if needed
func streamAudio(c *fiber.Ctx, meta *models.Meta) error {
	jobDir := utils.GetJobDir(meta.ID)
	audioPath := filepath.Join(jobDir, meta.Files.Audio.Name)

	// Check file exists
	if _, err := os.Stat(audioPath); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error: "Audio file not found",
		})
	}

	// Determine output format
	format := meta.Format
	contentType := utils.ContentTypeFromExt(format)

	// Generate filename
	filename := utils.GenerateOutputFilename(meta)
	encodedFilename := url.PathEscape(filename)

	// Set response headers
	c.Set("Content-Type", contentType)
	c.Set("Transfer-Encoding", "chunked")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename))
	c.Set("Cache-Control", "no-cache")

	// Check if transcoding is needed
	inputExt := filepath.Ext(meta.Files.Audio.Name)
	if len(inputExt) > 0 && inputExt[0] == '.' {
		inputExt = inputExt[1:]
	}

	var args []string

	if canCopyAudioStream(inputExt, format) {
		// No transcoding needed - just remux (very light CPU)
		log.Printf("[Stream %s] Audio remux: %s -> %s (copy)\n", meta.ID, inputExt, format)
		args = []string{
			"-y",
			"-i", audioPath,
			"-c:a", "copy",
			"-f", getFFmpegFormat(format),
			"pipe:1",
		}
	} else {
		// Transcoding needed (heavier CPU)
		codec := config.AudioCodecMap[format]
		if codec == "" {
			codec = "aac"
		}

		bitrate := meta.Bitrate
		if bitrate == "" {
			bitrate = "192k"
		}

		log.Printf("[Stream %s] Audio transcode: %s -> %s (codec: %s, bitrate: %s)\n", meta.ID, inputExt, format, codec, bitrate)

		args = []string{
			"-y",
			"-i", audioPath,
			"-vn",
			"-c:a", codec,
		}

		// Add bitrate for lossy codecs
		if codec != "pcm_s16le" && codec != "flac" {
			args = append(args, "-b:a", bitrate)
		}

		args = append(args, "-f", getFFmpegFormat(format), "pipe:1")
	}

	return runFFmpegStream(c, args, meta.ID)
}

// runFFmpegStream executes FFmpeg and pipes output to HTTP response
func runFFmpegStream(c *fiber.Ctx, args []string, jobID string) error {
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr // Log FFmpeg errors

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[Stream %s] Failed to create stdout pipe: %v\n", jobID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to start stream",
		})
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[Stream %s] Failed to start FFmpeg: %v\n", jobID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error: "Failed to start stream",
		})
	}

	// Stream FFmpeg output to client
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() {
			stdout.Close()
			cmd.Wait()
			log.Printf("[Stream %s] Stream completed\n", jobID)
		}()

		buf := make([]byte, 64*1024) // 64KB buffer
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					log.Printf("[Stream %s] Write error (client disconnected?): %v\n", jobID, writeErr)
					cmd.Process.Kill()
					return
				}
				w.Flush()
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("[Stream %s] Read error: %v\n", jobID, err)
				}
				return
			}
		}
	})

	return nil
}

// getFFmpegFormat returns the FFmpeg format name for a given extension
func getFFmpegFormat(ext string) string {
	switch ext {
	case "mp4":
		return "mp4"
	case "webm":
		return "webm"
	case "mkv":
		return "matroska"
	case "mp3":
		return "mp3"
	case "m4a":
		return "ipod" // FFmpeg uses "ipod" for m4a
	case "opus":
		return "opus"
	case "wav":
		return "wav"
	case "flac":
		return "flac"
	default:
		return ext
	}
}

// canCopyAudioStream checks if audio can be copied without re-encoding
func canCopyAudioStream(inputExt, outputFormat string) bool {
	if inputExt == outputFormat {
		return true
	}
	if (inputExt == "m4a" || inputExt == "mp4") && (outputFormat == "m4a" || outputFormat == "mp4") {
		return true
	}
	if inputExt == "webm" && outputFormat == "opus" {
		return true
	}
	return false
}
