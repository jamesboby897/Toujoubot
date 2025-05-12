package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"discord-youtube-bot/pkg/YT"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// Bot represents the Discord bot application
type Bot struct {
	session          *discordgo.Session
	voiceConnections map[string]*discordgo.VoiceConnection // Map of guild ID to voice connection
}

// AudioProcessor handles audio processing operations
type AudioProcessor struct {
	ytDlpPath  string
	audioCache map[string]string // Map of YouTube video ID to DCA file path
}

// YouTubeVideoInfo holds the JSON data returned by yt-dlp
type YouTubeVideoInfo struct {
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	ID       string `json:"id"`
	MediaURL string `json:"media_url"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get Discord token from .env
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("No Discord token found in .env file")
	}

	// Create audio directory if it doesn't exist
	if _, err := os.Stat("audio"); os.IsNotExist(err) {
		os.Mkdir("audio", 0755)
		fmt.Println("Created audio directory")
	}

	// Initialize audio processor
	processor := &AudioProcessor{
		ytDlpPath:  "cmd/yt-dlp/yt-dlp",
		audioCache: make(map[string]string),
	}

	// Load existing audio files into cache
	processor.loadAudioCache()

	// Update yt-dlp on startup
	processor.updateYtDlp()

	// Schedule yt-dlp updates every 24 hours
	go processor.scheduleYtDlpUpdates()

	// Create Discord session
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session:", err)
	}

	// Initialize bot
	bot := &Bot{
		session:          dg,
		voiceConnections: make(map[string]*discordgo.VoiceConnection),
	}

	// Register message handler
	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bot.handleMessage(s, m, processor)
	})

	// Open a websocket connection to Discord
	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection:", err)
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	// Wait here until CTRL-C or other term signal is received
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc

	// Disconnect from all voice channels
	for _, vc := range bot.voiceConnections {
		vc.Disconnect()
	}

	// Cleanly close down the Discord session
	dg.Close()
}

// handleMessage processes incoming Discord messages
func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate, processor *AudioProcessor) {
	// Ignore messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if message contains "-p" followed by a query
	if strings.Contains(m.Content, "-p") {
		// Extract the query (everything after "-p")
		parts := strings.SplitN(m.Content, "-p", 2)
		if len(parts) < 2 {
			return
		}
		query := strings.TrimSpace(parts[1])
		if query == "" {
			s.ChannelMessageSend(m.ChannelID, "Please provide a YouTube link or search query after -p")
			return
		}

		// Check if it's a YouTube link
		youtubeRegex := regexp.MustCompile(`(https?:\/\/)?(www\.)?(youtube\.com\/watch\?v=|youtu\.?be\/)([-\w]{11})`)
		matches := youtubeRegex.FindStringSubmatch(query)

		var videoID string
		if len(matches) >= 5 {
			videoID = matches[4]
			log.Printf("Detected YouTube link with video ID: %s", videoID)
		} else {
			log.Printf("No YouTube link detected, treating as search query: %s", query)
		}

		// Find the user's voice channel
		voiceChannelID, err := b.findUserVoiceChannel(s, m.GuildID, m.Author.ID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "You need to be in a voice channel for me to play audio.")
			return
		}

		// Check if audio already exists in cache (only if we have a videoID)
		if videoID != "" {
			dcaFilePath, exists := processor.audioCache[videoID]
			if exists {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Found cached audio for video ID: %s. Playing now...", videoID))

				// Join voice channel and play the audio from file
				go b.playAudio(s, m.GuildID, voiceChannelID, m.ChannelID, dcaFilePath)
				return
			}
		}

		// Send a confirmation message
		if videoID != "" {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Processing audio from YouTube video: %s", query))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Searching for: %s", query))
		}

		// Process the audio asynchronously
		go func() {
			// Download and process the audio, getting a stream and file path
			audioStream, dcaFilePath, err := processor.processYouTubeAudio(query, videoID)
			if err != nil {
				errorMsg := fmt.Sprintf("Error processing audio: %s", err)
				s.ChannelMessageSend(m.ChannelID, errorMsg)
				return
			}

			// Extract videoID from the file path if it wasn't provided
			if videoID == "" {
				baseFilename := filepath.Base(dcaFilePath)
				videoIDRegex := regexp.MustCompile(`([\w-]{11})\.dca$`)
				matches := videoIDRegex.FindStringSubmatch(baseFilename)
				if len(matches) >= 2 {
					videoID = matches[1]
				}
			}

			// Add to cache if we have a videoID
			if videoID != "" {
				processor.audioCache[videoID] = dcaFilePath
				log.Printf("Added audio for video ID %s to cache", videoID)
			}

			// Join voice channel and play the audio directly from the stream
			b.playAudio(s, m.GuildID, voiceChannelID, m.ChannelID, audioStream)
		}()
	}
}

// findUserVoiceChannel finds the voice channel a user is currently in
func (b *Bot) findUserVoiceChannel(s *discordgo.Session, guildID, userID string) (string, error) {
	// Get guild voice states
	guild, err := s.State.Guild(guildID)
	if err != nil {
		return "", fmt.Errorf("could not find guild: %w", err)
	}

	// Find the voice channel the user is in
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID, nil
		}
	}

	return "", fmt.Errorf("user not in a voice channel")
}

// playAudio joins a voice channel and plays audio from a DCA file or reader
func (b *Bot) playAudio(s *discordgo.Session, guildID, voiceChannelID, textChannelID string, audioSource interface{}) {
	// Check if we're already connected to this guild
	vc, exists := b.voiceConnections[guildID]
	if exists {
		// If we're in a different channel, disconnect first
		if vc.ChannelID != voiceChannelID {
			s.ChannelMessageSend(textChannelID, "I'm already in another channel")
		}
	}

	// Join the voice channel if not already connected
	if !exists || vc.ChannelID != voiceChannelID {
		var err error
		vc, err = s.ChannelVoiceJoin(guildID, voiceChannelID, false, true)
		if err != nil {
			s.ChannelMessageSend(textChannelID, fmt.Sprintf("Error joining voice channel: %s", err))
			return
		}
		b.voiceConnections[guildID] = vc
	}

	var reader io.Reader
	var closeFunc func()

	// Determine the type of audio source
	switch source := audioSource.(type) {
	case string:
		// It's a file path
		file, err := os.Open(source)
		if err != nil {
			s.ChannelMessageSend(textChannelID, fmt.Sprintf("Error opening DCA file: %s", err))
			return
		}
		reader = file
		closeFunc = func() { file.Close() }
	case io.Reader:
		// It's already a reader
		reader = source
		// If it's also a closer, set up the close function
		if closer, ok := source.(io.Closer); ok {
			closeFunc = func() { closer.Close() }
		} else {
			closeFunc = func() {}
		}
	default:
		s.ChannelMessageSend(textChannelID, "Invalid audio source type")
		return
	}

	// Make sure we close the reader when done
	defer closeFunc()

	vc.Speaking(true)
	defer vc.Speaking(false)

	for {
		var frameLen uint16
		err := binary.Read(reader, binary.LittleEndian, &frameLen)
		if err == io.EOF {
			break
		}
		if err != nil {
			s.ChannelMessageSend(textChannelID, fmt.Sprintf("Error reading audio frame: %s", err))
			break
		}

		frame := make([]byte, frameLen)
		_, err = io.ReadFull(reader, frame)
		if err != nil {
			s.ChannelMessageSend(textChannelID, fmt.Sprintf("Error reading audio data: %s", err))
			break
		}

		vc.OpusSend <- frame
	}
	vc.Disconnect()
	delete(b.voiceConnections, guildID)
}

// updateYtDlp updates the yt-dlp executable
func (p *AudioProcessor) updateYtDlp() {
	fmt.Println("Updating yt-dlp...")
	// Check if yt-dlp exists
	if _, err := os.Stat(p.ytDlpPath); os.IsNotExist(err) {
		log.Printf("Warning: yt-dlp not found at %s. Please make sure it's installed.", p.ytDlpPath)
		return
	}

	cmd := exec.Command(p.ytDlpPath, "-U")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error updating yt-dlp: %s\n%s", err, string(output))
		return
	}
	fmt.Printf("yt-dlp update result: %s\n", string(output))
}

// scheduleYtDlpUpdates schedules periodic updates for yt-dlp
func (p *AudioProcessor) scheduleYtDlpUpdates() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		p.updateYtDlp()
	}
}

// loadAudioCache loads existing audio files into the cache
func (p *AudioProcessor) loadAudioCache() {
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
			videoIDRegex := regexp.MustCompile(`([\w-]{11})\.(dca)$`)
			matches := videoIDRegex.FindStringSubmatch(file.Name())
			if len(matches) >= 2 {
				videoID := matches[1]
				p.audioCache[videoID] = filepath.Join("audio", file.Name())
				fmt.Printf("Cached audio file for video ID %s: %s\n", videoID, file.Name())
			}
		}
	}

	fmt.Printf("Loaded %d audio files into cache\n", len(p.audioCache))
}

// processYouTubeAudio downloads and processes audio from a YouTube link or search query
// Returns a reader for streaming and the file path
func (p *AudioProcessor) processYouTubeAudio(query, videoID string) (io.Reader, string, error) {
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
	if _, err := os.Stat(p.ytDlpPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("yt-dlp not found at %s: %w", p.ytDlpPath, err)
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
	jsonCmd := exec.Command(p.ytDlpPath, "-f", "251/250/249", "--print", "{\"title\": \"%(title)s\", \"duration\": %(duration)s, \"id\": \"%(id)s\", \"media_url\": \"%(url)s\"}", ytQuery)
	jsonOutput, err := jsonCmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, "", fmt.Errorf("yt-dlp failed: %s\nStderr: %s", err, exitErr.Stderr)
		}
		return nil, "", fmt.Errorf("failed to get video info: %w", err)
	}

	// Parse the JSON output
	var videoInfo YouTubeVideoInfo
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
		multiWriter := &multiWriter{writers: []io.Writer{dcaFile, pipeWriter}}

		// Convert WebM to DCA using the WebmToDCA package with our multiWriter
		log.Printf("Converting WebM to DCA format")
		err := YT.ConvertWebmToDca(resp.Body, &writeCloserWrapper{multiWriter})
		if err != nil {
			log.Printf("Error converting WebM to DCA: %v", err)
			return
		}
		log.Printf("Finished converting WebM to DCA format for video ID: %s", videoID)
	}()

	return pipeReader, dcaFilePath, nil
}

// multiWriter is a custom writer that writes to multiple writers
type multiWriter struct {
	writers []io.Writer
}

func (w *multiWriter) Write(p []byte) (n int, err error) {
	for _, writer := range w.writers {
		n, err = writer.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

// writeCloserWrapper wraps an io.Writer to provide a Close method
type writeCloserWrapper struct {
	io.Writer
}

func (w *writeCloserWrapper) Close() error {
	// No-op close method
	return nil
}
