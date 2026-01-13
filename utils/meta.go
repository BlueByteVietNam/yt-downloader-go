package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"yt-downloader-go/config"
	"yt-downloader-go/models"
)

// GetJobDir returns the directory path for a job
func GetJobDir(jobID string) string {
	return filepath.Join(config.StorageDir, jobID)
}

// GetMetaPath returns the meta.json path for a job
func GetMetaPath(jobID string) string {
	return filepath.Join(GetJobDir(jobID), "meta.json")
}

// ReadMeta reads the meta.json file for a job
func ReadMeta(jobID string) (*models.Meta, error) {
	data, err := os.ReadFile(GetMetaPath(jobID))
	if err != nil {
		return nil, err
	}

	var meta models.Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// WriteMeta writes the meta.json file for a job
func WriteMeta(jobID string, meta *models.Meta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(GetMetaPath(jobID), data, 0644)
}

// UpdateMetaStatus updates the status field
func UpdateMetaStatus(jobID string, status string) error {
	meta, err := ReadMeta(jobID)
	if err != nil {
		return err
	}
	meta.Status = status
	return WriteMeta(jobID, meta)
}

// UpdateMetaError updates status to error with message
func UpdateMetaError(jobID string, errMsg string) error {
	meta, err := ReadMeta(jobID)
	if err != nil {
		return err
	}
	meta.Status = "error"
	meta.Error = errMsg
	return WriteMeta(jobID, meta)
}

// UpdateMetaOutput updates the output filename
func UpdateMetaOutput(jobID string, output string) error {
	meta, err := ReadMeta(jobID)
	if err != nil {
		return err
	}
	meta.Status = "done"
	meta.Output = output
	return WriteMeta(jobID, meta)
}

// CreateJobDir creates the job directory
func CreateJobDir(jobID string) error {
	return os.MkdirAll(GetJobDir(jobID), 0755)
}

// DeleteJobDir deletes the job directory and all contents
func DeleteJobDir(jobID string) error {
	return os.RemoveAll(GetJobDir(jobID))
}

// JobExists checks if a job directory exists
func JobExists(jobID string) bool {
	_, err := os.Stat(GetJobDir(jobID))
	return err == nil
}

// GetFileSize returns the size of a file, or 0 if it doesn't exist
func GetFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// CalculateProgress calculates download progress from file sizes
func CalculateProgress(meta *models.Meta) (int, *models.ProgressDetail) {
	if meta.Status == "done" {
		return 100, nil
	}
	if meta.Status == "error" {
		return 0, nil
	}
	if meta.Status == "processing" {
		return 85, nil
	}

	jobDir := GetJobDir(meta.ID)
	detail := &models.ProgressDetail{}

	if meta.OutputType == "video" && meta.Files.Video != nil && meta.Files.Audio != nil {
		// Video + Audio download
		videoTmp := filepath.Join(jobDir, meta.Files.Video.Name+".tmp")
		audioTmp := filepath.Join(jobDir, meta.Files.Audio.Name+".tmp")

		videoSize := GetFileSize(videoTmp)
		audioSize := GetFileSize(audioTmp)

		// Check if finished files exist
		videoFinal := filepath.Join(jobDir, meta.Files.Video.Name)
		audioFinal := filepath.Join(jobDir, meta.Files.Audio.Name)
		if GetFileSize(videoFinal) > 0 {
			videoSize = meta.Files.Video.Size
		}
		if GetFileSize(audioFinal) > 0 {
			audioSize = meta.Files.Audio.Size
		}

		videoProgress := 0
		audioProgress := 0
		if meta.Files.Video.Size > 0 {
			videoProgress = int(float64(videoSize) / float64(meta.Files.Video.Size) * 100)
		}
		if meta.Files.Audio.Size > 0 {
			audioProgress = int(float64(audioSize) / float64(meta.Files.Audio.Size) * 100)
		}

		detail.Video = min(videoProgress, 100)
		detail.Audio = min(audioProgress, 100)

		// Weighted progress: video 70%, audio 30%, download phase 80%
		progress := int((float64(detail.Video)*0.7 + float64(detail.Audio)*0.3) * 0.8)
		return min(progress, 80), detail
	} else if meta.Files.Audio != nil {
		// Audio only
		audioTmp := filepath.Join(jobDir, meta.Files.Audio.Name+".tmp")
		audioSize := GetFileSize(audioTmp)

		audioFinal := filepath.Join(jobDir, meta.Files.Audio.Name)
		if GetFileSize(audioFinal) > 0 {
			audioSize = meta.Files.Audio.Size
		}

		audioProgress := 0
		if meta.Files.Audio.Size > 0 {
			audioProgress = int(float64(audioSize) / float64(meta.Files.Audio.Size) * 100)
		}

		detail.Audio = min(audioProgress, 100)
		progress := int(float64(detail.Audio) * 0.8)
		return min(progress, 80), detail
	}

	return 0, nil
}
