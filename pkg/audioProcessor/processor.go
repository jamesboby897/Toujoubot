package audioProcessor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"discord-youtube-bot/pkg/models"
	"discord-youtube-bot/pkg/utils"
)

// Processor handles audio processing operations
type Processor struct {
	YtDlpPath  string
	AudioCache map[string]string // Map of YouTube video ID to DCA file path
}

// NewProcessor creates a new audio processor
func NewProcessor(ytDlpPath string) *Processor {
	return &Processor{
		YtDlpPath:  ytDlpPath,
		AudioCache: make(map[string]string),
	}
}

// UpdateYtDlp updates the yt-dlp executable
func (p *Processor) UpdateYtDlp() {
	fmt.Println("Updating yt-dlp...")
	// Check if yt-dlp exists
	if _, err := os.Stat(p.YtDlpPath); os.IsNotExist(err) {
		log.Printf("Warning: yt-dlp not found at %s. Please make sure it's installed.", p.YtDlpPath)
		return
	}

	cmd := exec.Command(p.YtDlpPath, "-U")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error updating yt-dlp: %s\n%s", err, string(output))
		return
	}
	fmt.Printf("yt-dlp update result: %s\n", string(output))
}

// ScheduleYtDlpUpdates schedules periodic updates for yt-dlp
func (p *Processor) ScheduleYtDlpUpdates() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		p.UpdateYtDlp()
	}
}

// LoadAudioCache loads existing audio files into the cache
func (p *Processor) LoadAudioCache() {
	// Read the audio directory
	files, err := os.ReadDir("audio")
	if err != nil {
		log.Printf("Error reading audio directory: %s", err)
		return
	}

	// Process each .dca file
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".dca") {
			// Extract video ID from filename if possible
			videoID, found := utils.ExtractVideoIDFromFilename(file.Name())
			if found {
				p.AudioCache[videoID] = filepath.Join("audio", file.Name())
				fmt.Printf("Cached audio file for video ID %s: %s\n", videoID, file.Name())
			}
		}
	}

	fmt.Printf("Loaded %d audio files into cache\n", len(p.AudioCache))
}

// ProcessYouTubeAudio downloads and processes audio from a YouTube link or search query
// Returns a reader for streaming and the file path
func (p *Processor) ProcessYouTubeAudio(query, videoID string) (io.Reader, string, error) {
	// If videoID is provided, check if the file already exists in cache
	if videoID != "" {
		dcaFilePath := filepath.Join("audio", videoID+".dca")
		if _, err := os.Stat(dcaFilePath); err == nil {
			// File exists, just return the path
			file, err := os.Open(dcaFilePath)
			if err != nil {
				return nil, "", fmt.Errorf("failed to open existing DCA file: %w", err)
			}
			return file, dcaFilePath, nil
		}
	}

	// Check if yt-dlp exists
	if _, err := os.Stat(p.YtDlpPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("yt-dlp not found at %s: %w", p.YtDlpPath, err)
	}

	// Determine if the query is a URL or a search term
	var ytQuery string
	if strings.HasPrefix(query, "http") {
		ytQuery = query
	} else {
		ytQuery = "ytsearch:" + query
	}

	// Get video info using yt-dlp's JSON output
	log.Printf("Getting video info for query: %s", ytQuery)
	jsonCmd := exec.Command(p.YtDlpPath, "-f", "251/250/249", "--print", "{\"title\": \"%(title)s\", \"duration\": %(duration)s, \"id\": \"%(id)s\", \"media_url\": \"%(url)s\"}", ytQuery)
	jsonOutput, err := jsonCmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, "", fmt.Errorf("yt-dlp failed: %s\nStderr: %s", err, exitErr.Stderr)
		}
		return nil, "", fmt.Errorf("failed to get video info: %w", err)
	}

	// Parse the JSON output
	var videoInfo models.YouTubeVideoInfo
	err = json.Unmarshal(jsonOutput, &videoInfo)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse video info: %w\nOutput: %s", err, string(jsonOutput))
	}

	log.Printf("Found video: %s (ID: %s, Duration: %d seconds)", videoInfo.Title, videoInfo.ID, videoInfo.Duration)

	// If videoID was not provided, use the one from the JSON
	if videoID == "" {
		videoID = videoInfo.ID
	}

	// Create the DCA file path
	dcaFilePath := filepath.Join("audio", videoID+".dca")

	// Create a pipe for streaming
	pipeReader, pipeWriter := io.Pipe()

	// Create the DCA file
	dcaFile, err := os.Create(dcaFilePath)
	if err != nil {
		pipeReader.Close()
		return nil, "", fmt.Errorf("failed to create DCA file: %w", err)
	}

	// Get the audio stream from the media URL
	log.Printf("Downloading audio stream from media URL")
	resp, err := http.Get(videoInfo.MediaURL)
	if err != nil {
		dcaFile.Close()
		pipeReader.Close()
		return nil, "", fmt.Errorf("failed to get media stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		dcaFile.Close()
		pipeReader.Close()
		resp.Body.Close()
		return nil, "", fmt.Errorf("failed to get media stream: HTTP status %d", resp.StatusCode)
	}

	// Create a multiWriter to write to both the file and the pipe
	go func() {
		defer pipeWriter.Close()
		defer dcaFile.Close()
		defer resp.Body.Close()

		// Create a custom writer that writes to both outputs
		multiWriter := &models.MultiWriter{Writers: []io.Writer{dcaFile, pipeWriter}}

		// Convert WebM to DCA using the WebmToDCA package with our multiWriter
		log.Printf("Converting WebM to DCA format")
		err := convertWebmToDca(resp.Body, &models.WriteCloserWrapper{Writer: multiWriter})
		if err != nil {
			log.Printf("Error converting WebM to DCA: %v", err)
			return
		}
		log.Printf("Finished converting WebM to DCA format for video ID: %s", videoID)
	}()

	return pipeReader, dcaFilePath, nil
}
