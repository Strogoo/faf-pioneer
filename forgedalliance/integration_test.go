package forgedalliance

import (
	"fmt"
	"log"
	"testing"
)

func TestStandalone(t *testing.T) {
	gpgNetServer := NewGpgNetServer(31000)

	gameToAdapter := make(chan *GpgMessage)
	adapterToGame := make(chan *GpgMessage)

	go gpgNetServer.Listen(gameToAdapter, adapterToGame)

	fmt.Println("GpgNet TCP server started, please start the game now")

	gameStateLobby := <-gameToAdapter

	var message GpgMessage = &HostGameMessage{
		Command: "HostGame",
		MapName: "SCMP_0014",
	}
	adapterToGame <- &message

	log.Printf("GameStateLobby: %v", gameStateLobby)

}
