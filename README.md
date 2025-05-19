# Toujoubot: Discord YouTube Audio Downloader and Player Bot

This Discord bot scans messages for "-p" followed by a YouTube link or search query, then downloads the audio using yt-dlp, extracts Opus frames from the WebM file, saves them as a DCA file, and plays the audio in the user's voice channel.

## Prerequisites

1. Go (1.16 or later)
2. yt-dlp installed in the cmd/yt-dlp path
3. Discord Bot Token

## Setup

1. Make sure yt-dlp is available at cmd/yt-dlp
   - The bot will automatically update yt-dlp on startup and every 24 hours

2. Update the `.env` file with your Discord bot token:
   ```
   DISCORD_TOKEN=your_discord_bot_token_here
   ```

3. Compile Toujoubot:
   ```
   make
   ```

4. Run the bot:
   ```
   ./toujoubot
   ```

## Usage

1. Join a voice channel in Discord
2. In any text channel where the bot is present, send a message containing "-p" followed by a YouTube link or search query. For example:
   ```
   Hey, check out this song -p https://www.youtube.com/watch?v=dQw4w9WgXcQ
   ```
   Or search for a video:
   ```
   -p never gonna give you up
   ```
3. The bot will:
   - Join your voice channel
   - Download the audio from the YouTube link or search for the video
   - Extract Opus frames directly from the WebM stream
   - Save the audio as a DCA file in the `audio` folder
   - Play the audio in your voice channel

## Features

- **YouTube Search**: The bot can search for videos if you don't provide a direct link
- **Audio Caching**: The bot caches downloaded audio files by YouTube video ID, so it doesn't need to download the same audio twice
- **Direct Media Streaming**: The bot uses yt-dlp to get the media URL and streams it directly
- **Voice Channel Integration**: The bot automatically joins the user's voice channel and plays the audio
- **Automatic Updates**: yt-dlp is updated on bot startup and every 24 hours
