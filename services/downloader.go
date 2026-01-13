package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"yt-downloader-go/config"
)

// DownloadStats tracks download statistics
type DownloadStats struct {
	StartTime    time.Time
	ChunksTotal  int
	ChunksDone   int32
	BytesTotal   int64
	BytesDone    int64
	FailedChunks int32
	RetryCount   int32
}

// Download downloads a file using parallel chunked requests
func Download(ctx context.Context, downloadURL string, destPath string, totalSize int64) error {
	// Small files: single request
	if totalSize <= config.ChunkSize {
		return downloadSingle(ctx, downloadURL, destPath, totalSize)
	}

	// Large files: parallel chunked download
	return downloadChunked(ctx, downloadURL, destPath, totalSize)
}

// downloadSingle downloads small files in a single request
func downloadSingle(ctx context.Context, downloadURL string, destPath string, totalSize int64) error {
	tmpPath := destPath + ".tmp"

	data, err := fetchRange(ctx, downloadURL, 0, totalSize-1, 0)
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return os.Rename(tmpPath, destPath)
}

// downloadChunked downloads large files using parallel workers
func downloadChunked(ctx context.Context, downloadURL string, destPath string, totalSize int64) error {
	tmpPath := destPath + ".tmp"

	// Calculate chunks
	numChunks := int((totalSize + config.ChunkSize - 1) / config.ChunkSize)

	stats := &DownloadStats{
		StartTime:   time.Now(),
		ChunksTotal: numChunks,
		BytesTotal:  totalSize,
	}

	// Create channels for work distribution
	type chunkResult struct {
		index int
		data  []byte
		err   error
	}

	results := make([]chan chunkResult, numChunks)
	for i := 0; i < numChunks; i++ {
		results[i] = make(chan chunkResult, 1)
	}

	// Start workers
	var wg sync.WaitGroup
	chunkIndex := int32(-1)

	for w := 0; w < config.Threads; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				// Get next chunk
				idx := int(atomic.AddInt32(&chunkIndex, 1))
				if idx >= numChunks {
					return
				}

				// Check context
				select {
				case <-ctx.Done():
					results[idx] <- chunkResult{index: idx, err: ctx.Err()}
					return
				default:
				}

				// Calculate byte range
				start := int64(idx) * config.ChunkSize
				end := start + config.ChunkSize - 1
				if end >= totalSize {
					end = totalSize - 1
				}

				// Download with retries
				var data []byte
				var err error
				for retry := 0; retry < config.MaxRetries; retry++ {
					data, err = fetchRange(ctx, downloadURL, start, end, idx)
					if err == nil {
						break
					}

					atomic.AddInt32(&stats.RetryCount, 1)

					// Don't retry on 403
					if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode == 403 {
						atomic.AddInt32(&stats.FailedChunks, 1)
						break
					}

					if retry < config.MaxRetries-1 {
						time.Sleep(config.RetryDelay * time.Duration(retry+1))
					}
				}

				if err != nil {
					atomic.AddInt32(&stats.FailedChunks, 1)
					results[idx] <- chunkResult{index: idx, err: err}
					return
				}

				atomic.AddInt32(&stats.ChunksDone, 1)
				atomic.AddInt64(&stats.BytesDone, int64(len(data)))
				results[idx] <- chunkResult{index: idx, data: data}
			}
		}(w)
	}

	// Writer goroutine - writes chunks in order
	writeErr := make(chan error, 1)
	go func() {
		file, err := os.Create(tmpPath)
		if err != nil {
			writeErr <- fmt.Errorf("failed to create file: %w", err)
			return
		}
		defer file.Close()

		for i := 0; i < numChunks; i++ {
			result := <-results[i]
			if result.err != nil {
				writeErr <- result.err
				return
			}

			if _, err := file.Write(result.data); err != nil {
				writeErr <- fmt.Errorf("failed to write chunk %d: %w", i, err)
				return
			}

			// Clear data for GC
			results[i] = nil
		}

		writeErr <- nil
	}()

	// Wait for all workers
	wg.Wait()

	// Check write result
	if err := <-writeErr; err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Rename temp to final
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// HTTPError represents an HTTP error
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// fetchRange fetches a byte range from URL
func fetchRange(ctx context.Context, downloadURL string, start, end int64, chunkIndex int) ([]byte, error) {
	// Add range to URL as query parameter
	rangeURL := fmt.Sprintf("%s&range=%d-%d", downloadURL, start, end)

	req, err := http.NewRequestWithContext(ctx, "GET", rangeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Referer", "https://www.youtube.com/")

	// New client per request = new random IPv6
	client := config.NewIPv6Client(config.ChunkTimeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// DownloadFile is a simpler download function for small files
func DownloadFile(ctx context.Context, downloadURL string, destPath string) error {
	req, err := http.NewRequestWithConưtext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := config.NewIPv6Client(config.ChunkTimeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &HTTPError{StatusCode: resp.StatusCode, Message: "download failed"}
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(destPath)
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
