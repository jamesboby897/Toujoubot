package models

import (
	"io"

	"github.com/bwmarrin/discordgo"
)

// Bot represents the Discord bot application
type Bot struct {
	Session          *discordgo.Session
	VoiceConnections map[string]*discordgo.VoiceConnection // Map of guild ID to voice connection
}

// AudioProcessor handles audio processing operations
type AudioProcessor struct {
	YtDlpPath  string
	AudioCache map[string]string // Map of YouTube video ID to DCA file path
}

// YouTubeVideoInfo holds the JSON data returned by yt-dlp
type YouTubeVideoInfo struct {
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	ID       string `json:"id"`
	MediaURL string `json:"media_url"`
}

// MultiWriter is a custom writer that writes to multiple writers
type MultiWriter struct {
	Writers []io.Writer
}

func (w *MultiWriter) Write(p []byte) (n int, err error) {
	for _, writer := range w.Writers {
		n, err = writer.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

// WriteCloserWrapper wraps an io.Writer to provide a Close method
type WriteCloserWrapper struct {
	io.Writer
}

func (w *WriteCloserWrapper) Close() error {
	// No-op close method
	return nil
}
