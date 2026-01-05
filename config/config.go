package config

import (
	"context"
	"crypto/rand"
	"net"
	"net/http"
	"time"
)

const (
	// Server
	Port = 5001

	// Storage
	StorageDir = "./storage"

	// Download settings
	Threads      = 4
	ChunkSize    = 10_000_000 // 10MB
	MaxRetries   = 3
	RetryDelay   = 100 * time.Millisecond
	ChunkTimeout = 10 * time.Second

	// Proxy (for Extract API)
	ExtractProxyURL = "http://64.176.170.104:21589"

	// Extract API
	ExtractAPIBase    = "http://168.119.14.32:8300/api/youtube/video"
	ExtractAPITimeout = 15 * time.Second

	// Cleanup
	CleanupInterval  = "0 * * * *" // Every hour
	MaxJobAge        = 1 * time.Hour
	CleanupBatchSize = 5000 // Must handle 100k+ jobs/day

	// Job ID
	JobIDLength = 21
	JobIDRegex  = `^[a-zA-Z0-9_-]{21}$`

	// Limits
	MaxTrimDuration = 24 * time.Hour
)

// Supported formats
var (
	VideoFormats = []string{"mp4", "webm", "mkv"}
	AudioFormats = []string{"mp3", "m4a", "wav", "opus", "flac"}
	Qualities    = []string{"2160p", "1440p", "1080p", "720p", "480p", "360p", "144p"}
	OSTypes      = []string{"ios", "android", "macos", "windows", "linux"}
)

// Quality to height mapping
var QualityToHeight = map[string]int{
	"2160p": 2160,
	"1440p": 1440,
	"1080p": 1080,
	"720p":  720,
	"480p":  480,
	"360p":  360,
	"144p":  144,
}

// Height to quality mapping
var HeightToQuality = map[int]string{
	2160: "2160p",
	1440: "1440p",
	1080: "1080p",
	720:  "720p",
	480:  "480p",
	360:  "360p",
	144:  "144p",
}

// Device profiles
type DeviceProfile struct {
	MaxQuality  string
	VideoCodecs []string
	AudioCodecs []string
}

var DeviceProfiles = map[string]DeviceProfile{
	"ios": {
		MaxQuality:  "1080p",
		VideoCodecs: []string{"avc1"},
		AudioCodecs: []string{"mp4a"},
	},
	"android": {
		MaxQuality:  "2160p",
		VideoCodecs: []string{"av01", "vp9", "avc1"},
		AudioCodecs: []string{"opus", "mp4a"},
	},
	"macos": {
		MaxQuality:  "1080p",
		VideoCodecs: []string{"avc1"},
		AudioCodecs: []string{"mp4a"},
	},
	"windows": {
		MaxQuality:  "2160p",
		VideoCodecs: []string{"av01", "vp9", "avc1"},
		AudioCodecs: []string{"opus", "mp4a"},
	},
	"linux": {
		MaxQuality:  "2160p",
		VideoCodecs: []string{"av01", "vp9", "avc1"},
		AudioCodecs: []string{"opus", "mp4a"},
	},
}

// Default profile
var DefaultProfile = DeviceProfile{
	MaxQuality:  "1080p",
	VideoCodecs: []string{"avc1"},
	AudioCodecs: []string{"mp4a"},
}

// FFmpeg codec mappings
var AudioCodecMap = map[string]string{
	"mp3":  "libmp3lame",
	"m4a":  "aac",
	"mp4":  "aac",
	"wav":  "pcm_s16le",
	"opus": "libopus",
	"flac": "flac",
	"webm": "libopus",
}

var VideoCodecMap = map[string]string{
	"mp4":  "libx264",
	"mkv":  "libx264",
	"webm": "libvpx-vp9",
}

// MIME type to extension mapping
var MimeToExt = map[string]string{
	"video/mp4":   "mp4",
	"video/webm":  "webm",
	"audio/mp4":   "m4a",
	"audio/webm":  "webm",
	"audio/mpeg":  "mp3",
	"audio/ogg":   "ogg",
	"audio/opus":  "opus",
	"audio/flac":  "flac",
	"audio/wav":   "wav",
	"audio/x-wav": "wav",
}

// IPv6 rotation config - 2a01:4f8:a0:61c5::/64
var IPv6Prefix = [8]byte{0x2a, 0x01, 0x04, 0xf8, 0x00, 0xa0, 0x61, 0xc5}

// RandomIPv6 generates a random IPv6 address from the /64 subnet
func RandomIPv6() net.IP {
	ip := make([]byte, 16)
	copy(ip[:8], IPv6Prefix[:])
	rand.Read(ip[8:]) // Random 64 bits
	return ip
}

// NewIPv6Client creates an HTTP client with a random IPv6 source address
func NewIPv6Client(timeout time.Duration) *http.Client {
	localAddr := &net.TCPAddr{IP: RandomIPv6()}

	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					LocalAddr: localAddr,
					Timeout:   10 * time.Second,
				}
				return dialer.DialContext(ctx, "tcp6", addr)
			},
			DisableKeepAlives: true,
		},
		Timeout: timeout,
	}
}

// HTTP Clients
var ExtractClient *http.Client

func init() {
	// Extract API client (proxy passed as query parameter, not HTTP transport)
	ExtractClient = &http.Client{
		Timeout: ExtractAPITimeout,
	}
}
