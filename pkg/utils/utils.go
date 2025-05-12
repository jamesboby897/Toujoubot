package utils

import (
	"fmt"
	"os"
	"regexp"
)

// EnsureDirectoryExists ensures that a directory exists, creating it if necessary
func EnsureDirectoryExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
		fmt.Printf("Created directory: %s\n", path)
	}
	return nil
}

// ExtractVideoIDFromFilename extracts a YouTube video ID from a filename
func ExtractVideoIDFromFilename(filename string) (string, bool) {
	videoIDRegex := regexp.MustCompile(`([\w-]{11})\.(dca)$`)
	matches := videoIDRegex.FindStringSubmatch(filename)
	if len(matches) >= 2 {
		return matches[1], true
	}
	return "", false
}

// ExtractVideoIDFromURL extracts a YouTube video ID from a URL
func ExtractVideoIDFromURL(url string) (string, bool) {
	youtubeRegex := regexp.MustCompile(`(https?:\/\/)?(www\.)?(youtube\.com\/watch\?v=|youtu\.?be\/)([-\w]{11})`)
	matches := youtubeRegex.FindStringSubmatch(url)
	if len(matches) >= 5 {
		return matches[4], true
	}
	return "", false
}
