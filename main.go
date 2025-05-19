package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"discord-youtube-bot/pkg/audioProcessor"
	"discord-youtube-bot/pkg/bot"
	"discord-youtube-bot/pkg/utils"

	"github.com/joho/godotenv"
)

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
	err = utils.EnsureDirectoryExists("audio")
	if err != nil {
		log.Fatal("Error creating audio directory:", err)
	}

	// Initialize audio processor
	processor := audioProcessor.NewProcessor()

	// Load existing audio files into cache
	processor.LoadAudioCache()

	// Update yt-dlp on startup
	processor.UpdateYtDlp()

	// Schedule yt-dlp updates every 24 hours
	go processor.ScheduleYtDlpUpdates()

	// Create and initialize bot
	discordBot, err := bot.New(token)
	if err != nil {
		log.Fatal("Error creating bot:", err)
	}

	// Start the bot
	err = discordBot.Start()
	if err != nil {
		log.Fatal("Error starting bot:", err)
	}

	// Wait here until CTRL-C or other term signal is received
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc

	// Stop the bot
	discordBot.Stop()
	fmt.Println("Bot has been shut down")
}
