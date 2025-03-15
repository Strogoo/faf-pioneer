package main

import (
	"faf-pioneer/adapter"
	"flag"
	"log/slog"
	"os"
)

func main() {
	// Define flags without default values (not passing a value will cause an error)
	userId := flag.Uint("user-id", 0, "The ID of the user")
	gameId := flag.Uint64("game-id", 0, "The ID of the game session")
	accessToken := flag.String("access-token", "", "The access token for authentication")
	apiRoot := flag.String("api-root", "https://api.faforever.com/ice", "The root uri of the icebreaker api")
	gpgNetPort := flag.Uint("gpgnet-port", 0, "The port which the game will connect to for exchanging GgpNet messages")
	gpgNetClientPort := flag.Uint("gpgnet-client-port", 0, "The port which on which the parent FAF client listens on")
	gameUdpPort := flag.Uint("game-udp-port", 0, "The port which the game will send/receive game data")

	// Parse the command-line flags
	flag.Parse()

	// Validate that the required flags are provided
	if *userId == 0 {
		slog.Error("Error: --user-id is required and must be a valid uint32.")
		os.Exit(1)
	}

	if *gameId == 0 {
		slog.Error("Error: --game-id is required and must be a valid uint64.")
		os.Exit(1)
	}

	if *accessToken == "" {
		slog.Error("Error: --access-token is required and cannot be empty.")
		os.Exit(1)
	}

	if *gpgNetPort == 0 {
		slog.Error("Error: --gpgnet-port is required and cannot be empty.")
		os.Exit(1)
	}

	adapter.Start(
		*userId,
		*gameId,
		*accessToken,
		*apiRoot,
		*gpgNetPort,
		*gpgNetClientPort,
		*gameUdpPort,
	)
}
