package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
)

func GetFileHash(url string) (string, error) {
	// Fetch the file from the URL
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("GetFileHash: %w", err)
	}
	defer resp.Body.Close()

	// Create a hash function based on the specified type
	h := sha256.New()

	// Copy the file content to the hash function
	if _, err := io.Copy(h, resp.Body); err != nil {
		return "", fmt.Errorf("GetFileHash: %w", err)
	}

	// Convert the hash to a string
	hash := fmt.Sprintf("%x", h.Sum(nil))
	return hash, nil
}
