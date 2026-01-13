package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"
	"yt-downloader-go/config"
)

// GenerateSignedURL creates a signed URL with token and expiration
func GenerateSignedURL(jobID, filename string) string {
	expires := time.Now().Add(config.SignedURLExpiration).Unix()
	token := generateToken(jobID, filename, expires)
	domain := getRandomDomain()
	return fmt.Sprintf("%s/files/%s/%s?token=%s&expires=%d", domain, jobID, filename, token, expires)
}

// getRandomDomain returns a random domain from the list
func getRandomDomain() string {
	domains := config.DownloadDomains
	if len(domains) == 0 {
		return ""
	}
	return domains[rand.Intn(len(domains))]
}

// ValidateSignedURL checks if the token is valid and not expired
func ValidateSignedURL(jobID, filename, token string, expires int64) bool {
	// Check if expired
	if time.Now().Unix() > expires {
		return false
	}

	// Validate token
	expectedToken := generateToken(jobID, filename, expires)
	return hmac.Equal([]byte(token), []byte(expectedToken))
}

// generateToken creates HMAC-SHA256 token
func generateToken(jobID, filename string, expires int64) string {
	data := fmt.Sprintf("%s:%s:%d", jobID, filename, expires)
	h := hmac.New(sha256.New, []byte(config.SignedURLSecret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ParseExpires converts expires string to int64
func ParseExpires(expiresStr string) (int64, error) {
	return strconv.ParseInt(expiresStr, 10, 64)
}
