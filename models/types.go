package models

// DownloadRequest represents the incoming download request
type DownloadRequest struct {
	URL    string       `json:"url"`
	OS     string       `json:"os,omitempty"`
	Output OutputConfig `json:"output"`
	Audio  AudioConfig  `json:"audio,omitempty"`
	Trim   *TrimConfig  `json:"trim,omitempty"`
}

type OutputConfig struct {
	Type    string `json:"type"`    // "video" or "audio"
	Format  string `json:"format"`  // mp4, webm, mkv, mp3, m4a, etc.
	Quality string `json:"quality,omitempty"` // 1080p, 720p, etc.
}

type AudioConfig struct {
	TrackID string `json:"trackId,omitempty"`
	Bitrate string `json:"bitrate,omitempty"` // 192k, 320k, etc.
}

type TrimConfig struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Accurate bool    `json:"accurate,omitempty"`
}

// DownloadResponse is returned when a job is created
type DownloadResponse struct {
	ID                  string  `json:"id"`
	Title               string  `json:"title"`
	Duration            float64 `json:"duration"`
	RequestedQuality    string  `json:"requestedQuality,omitempty"`
	SelectedQuality     string  `json:"selectedQuality,omitempty"`
	QualityChanged      bool    `json:"qualityChanged"`
	QualityChangeReason string  `json:"qualityChangeReason,omitempty"`
	NeedsReencode       bool    `json:"needsReencode"`
}

// Job status constants
const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusError     = "error"
)

// StatusResponse is returned when checking job status
type StatusResponse struct {
	Status      string          `json:"status"` // pending, completed, error
	Progress    int             `json:"progress"`
	Title       string          `json:"title,omitempty"`
	Duration    float64         `json:"duration,omitempty"`
	DownloadURL string          `json:"downloadUrl,omitempty"` // only when completed
	JobError    string          `json:"jobError,omitempty"`    // only when status is error
	Detail      *ProgressDetail `json:"detail,omitempty"`
}

type ProgressDetail struct {
	Video int `json:"video,omitempty"`
	Audio int `json:"audio,omitempty"`
}

// Meta represents job metadata stored in meta.json
type Meta struct {
	ID         string      `json:"id"`
	Status     string      `json:"status"` // pending, completed, error
	CreatedAt  int64       `json:"createdAt"`
	VideoID    string      `json:"videoId"`
	Title      string      `json:"title"`
	Duration   float64     `json:"duration"`
	Files      FilesInfo   `json:"files"`
	OutputType string      `json:"outputType"` // video or audio
	Format     string      `json:"format"`
	Quality    string      `json:"quality,omitempty"`
	Bitrate    string      `json:"bitrate,omitempty"`
	Trim       *TrimConfig `json:"trim,omitempty"`
	Output     string      `json:"output,omitempty"`
	StreamOnly bool        `json:"streamOnly,omitempty"` // true = skip merge, stream only
	Error      string      `json:"error,omitempty"`
}

type FilesInfo struct {
	Video *FileInfo `json:"video,omitempty"`
	Audio *FileInfo `json:"audio,omitempty"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ExtractResponse from YouTube Extract API
type ExtractResponse struct {
	Title        string   `json:"title"`
	Duration     float64  `json:"duration"`
	VideoStreams []Stream `json:"videoStreams"`
	AudioStreams []Stream `json:"audioStreams"`
}

// Stream represents a video or audio stream
type Stream struct {
	URL           string  `json:"url"`
	MimeType      string  `json:"mimeType"`
	Codec         string  `json:"codec,omitempty"`
	Quality       string  `json:"quality,omitempty"`
	QualityLabel  string  `json:"qualityLabel,omitempty"`
	Width         int     `json:"width,omitempty"`
	Height        int     `json:"height,omitempty"`
	Bitrate       float64 `json:"bitrate,omitempty"`
	ContentLength int64   `json:"fileSize,omitempty"`
	AudioTrackID  string  `json:"audioTrackId,omitempty"`
	IsOriginal    bool    `json:"isOriginal,omitempty"`
	FPS           int     `json:"fps,omitempty"`
}

// VideoSelectionResult contains the selected video stream and metadata
type VideoSelectionResult struct {
	Stream              *Stream
	SelectedQuality     string
	QualityChanged      bool
	QualityChangeReason string
	NeedsReencode       bool
}

// HealthResponse for health check
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// DeleteResponse for job deletion
type DeleteResponse struct {
	Deleted bool `json:"deleted"`
}
