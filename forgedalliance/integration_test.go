package forgedalliance

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
)

// This test starts the game and sends some mock lobby server messages to get it to initialize out of its blackscreen state
func TestStandalone(t *testing.T) {
	gpgNetServer := NewGpgNetServer(31000)

	gameToAdapter := make(chan *GpgMessage)
	adapterToGame := make(chan *GpgMessage)

	go gpgNetServer.Listen(gameToAdapter, adapterToGame)

	fmt.Println("GpgNet TCP server started, please start the game now")

	// Get the current user's home directory
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Error getting user home directory:", err)
		os.Exit(1)
	}
	fafDir := filepath.Join(usr.HomeDir, ".faforever/bin")

	cmd := exec.Command("/bin/sh", "./runfaf3.sh",
		"/init", "init.lua",
		"/nobugreport",
		"/gpgnet", "127.0.0.1:31000",
		"/mean", "1500.0",
		"/deviation", "500.0",
		"/country", "ES",
		"/numgames", "9999",
	)

	cmd.Dir = fafDir

	// Redirect output to terminal
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	// Run the command
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting Forged Alliance:", err)
		os.Exit(1)
	}

	fmt.Println("Forged Alliance started successfully!")

	// Receive GameState=Idle "hello" from game
	gameStateLobby := <-gameToAdapter

	// Send message to game to create the lobby and receive acknowledgement
	var createGameLobbyMessage GpgMessage = &CreateLobbyMessage{
		Command:          "CreateLobby",
		LobbyInitMode:    0,
		LobbyPort:        60000,
		LocalPlayerName:  "p4block",
		LocalPlayerId:    18746,
		UnknownParameter: 1,
	}
	adapterToGame <- &createGameLobbyMessage
	gameStateLobby = <-gameToAdapter

	// Send mapname (optional, it will use a fallback default map if empty)
	// This message is required else game is stuck on Connecting...
	var message GpgMessage = &HostGameMessage{
		Command: "HostGame",
		MapName: "",
	}
	adapterToGame <- &message
	gameStateLobby = <-gameToAdapter

	// Send more things if desired..
	// var message..

	log.Printf("GameStateLobby: %v", gameStateLobby)

}
