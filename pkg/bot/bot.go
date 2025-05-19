package bot

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"discord-youtube-bot/pkg/audioProcessor"
	"discord-youtube-bot/pkg/utils"

	"github.com/bwmarrin/discordgo"
)

// Bot represents the Discord bot application
type Bot struct {
	Session          *discordgo.Session
	VoiceConnections map[string]*discordgo.VoiceConnection // Map of guild ID to voice connection
}

// New creates a new Bot instance
func New(token string) (*Bot, error) {
	// Create Discord session
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	// Initialize bot
	bot := &Bot{
		Session:          dg,
		VoiceConnections: make(map[string]*discordgo.VoiceConnection),
	}

	// Register message handler
	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bot.HandleMessage(s, m)
	})

	return bot, nil
}

// Start starts the bot
func (b *Bot) Start() error {
	// Open a websocket connection to Discord
	err := b.Session.Open()
	if err != nil {
		return fmt.Errorf("error opening connection: %w", err)
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	return nil
}

// Stop stops the bot
func (b *Bot) Stop() {
	// Disconnect from all voice channels
	for _, vc := range b.VoiceConnections {
		vc.Disconnect()
	}

	// Cleanly close down the Discord session
	b.Session.Close()
}

// HandleMessage processes incoming Discord messages
func (b *Bot) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Create audio processor
	processor := audioProcessor.NewProcessor()

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
		videoID, isYouTubeURL := utils.ExtractVideoIDFromURL(query)
		if isYouTubeURL {
			log.Printf("Detected YouTube link with video ID: %s", videoID)
		} else {
			log.Printf("No YouTube link detected, treating as search query: %s", query)
			videoID = ""
		} // Find the user's voice channel
		voiceChannelID, err := b.FindUserVoiceChannel(s, m.GuildID, m.Author.ID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "You need to be in a voice channel for me to play audio.")
			return
		}

		// Check if audio already exists in cache (only if we have a videoID)
		if videoID != "" {
			dcaFilePath, exists := processor.AudioCache[videoID]
			if exists {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Found cached audio for video ID: %s. Playing now...", videoID))

				// Join voice channel and play the audio from file
				go b.PlayAudio(s, m.GuildID, voiceChannelID, m.ChannelID, dcaFilePath)
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
			audioStream, dcaFilePath, err := processor.ProcessYouTubeAudio(query, videoID)
			if err != nil {
				errorMsg := fmt.Sprintf("Error processing audio: %s", err)
				s.ChannelMessageSend(m.ChannelID, errorMsg)
				return
			}

			// Extract videoID from the file path if it wasn't provided
			if videoID == "" {
				baseFilename := filepath.Base(dcaFilePath)

				extractedID, found := utils.ExtractVideoIDFromFilename(baseFilename)
				if found {
					videoID = extractedID
				}
			}

			// Add to cache if we have a videoID
			if videoID != "" {
				processor.AudioCache[videoID] = dcaFilePath
				log.Printf("Added audio for video ID %s to cache", videoID)
			}

			// Join voice channel and play the audio directly from the stream
			b.PlayAudio(s, m.GuildID, voiceChannelID, m.ChannelID, audioStream)
		}()
	}
}

// FindUserVoiceChannel finds the voice channel a user is currently in
func (b *Bot) FindUserVoiceChannel(s *discordgo.Session, guildID, userID string) (string, error) {
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

// PlayAudio joins a voice channel and plays audio from a DCA file or reader
func (b *Bot) PlayAudio(s *discordgo.Session, guildID, voiceChannelID, textChannelID string, audioSource interface{}) {
	// Check if we're already connected to this guild
	vc, exists := b.VoiceConnections[guildID]
	if exists {
		// If we're in a different channel, disconnect first
		if vc.ChannelID != voiceChannelID {
			s.ChannelMessageSend(textChannelID, "I'm already in another channel")
			return
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
		b.VoiceConnections[guildID] = vc
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
	delete(b.VoiceConnections, guildID)
}
