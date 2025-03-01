package main

import (
	"faf-pioneer/icebreaker"
	"faf-pioneer/webrtc"
	"flag"
	"fmt"
	pionwebrtc "github.com/pion/webrtc/v4"
	"log"
	"strings"
)

func main() {
	// Define flags without default values (not passing a value will cause an error)
	userId := flag.Uint("user-id", 0, "The ID of the user")
	gameId := flag.Uint64("game-id", 0, "The ID of the game session")
	accessToken := flag.String("access-token", "", "The access token for authentication")
	apiRoot := flag.String("api-root", "https://api.faforever.com/ice", "The root uri of the icebreaker api")

	// Parse the command-line flags
	flag.Parse()

	// Validate that the required flags are provided
	if *userId == 0 {
		log.Fatalf("Error: --user-id is required and must be a valid uint32.")
	}

	if *gameId == 0 {
		log.Fatalf("Error: --game-id is required and must be a valid uint64.")
	}

	if *accessToken == "" {
		log.Fatalf("Error: --access-token is required and cannot be empty.")
	}

	// Gather ICE servers and listen for WebRTC events
	icebreakerClient := icebreaker.NewClient(*apiRoot, *gameId, *accessToken)
	sessionGameResponse, err := icebreakerClient.GetGameSession()

	if err != nil {
		log.Fatal("Could not query turn servers: ", err)
	}

	channel := make(chan icebreaker.EventMessage)
	go icebreakerClient.Listen(channel)

	turnServer := make([]pionwebrtc.ICEServer, len(sessionGameResponse.Servers))
	for i, server := range sessionGameResponse.Servers {
		turnServer[i] = pionwebrtc.ICEServer{
			Username:       server.Username,
			Credential:     server.Credential,
			CredentialType: pionwebrtc.ICECredentialTypePassword,
			URLs:           make([]string, len(server.Urls)),
		}

		for j, url := range server.Urls {
			// for Java being Java reasons we unfortunately raped the URLs and need to convert it back
			turnServer[i].URLs[j] = strings.ReplaceAll(url, "://", ":")
		}
	}

	// WebRTC setup
	peers := make(map[uint]*webrtc.Peer)

	var peerUdpPort uint16 = 18000 // TODO: Pick a "free random one"
	var gameUdpPort uint16 = 19000 // TODO: Get via GpgNet from the game

	for msg := range channel {
		switch event := msg.(type) {
		case *icebreaker.ConnectedMessage:
			log.Printf("Connecting to peer: %s\n", event)

			peer, err := webrtc.CreatePeer(true, event.SenderID, turnServer, peerUdpPort, gameUdpPort, func(description *pionwebrtc.SessionDescription, candidates []*pionwebrtc.ICECandidate) {
				err := icebreakerClient.SendEvent(
					icebreaker.CandidatesMessage{
						EventType:   "candidates",
						GameID:      *gameId,
						SenderID:    *userId,
						RecipientID: &event.SenderID,
						Session:     description,
						Candidates:  candidates,
					})

				if err != nil {
					log.Printf("Failed to send candidates: %s\n", err)
				}
			})

			if err != nil {
				panic(err)
			}

			peers[event.SenderID] = peer
		case *icebreaker.CandidatesMessage:
			fmt.Printf("Received CandidatesMessage: %s\n", event)
			peer := peers[event.SenderID]

			if peer == nil {
				peer, err = webrtc.CreatePeer(false, event.SenderID, turnServer, peerUdpPort, gameUdpPort, func(description *pionwebrtc.SessionDescription, candidates []*pionwebrtc.ICECandidate) {
					err := icebreakerClient.SendEvent(
						icebreaker.CandidatesMessage{
							EventType:   "candidates",
							GameID:      *gameId,
							SenderID:    *userId,
							RecipientID: &event.SenderID,
							Session:     description,
							Candidates:  candidates,
						})

					if err != nil {
						log.Printf("Failed to send candidates: %s\n", err)
					}
				})
				if err != nil {
					panic(err)
				}

				peers[event.SenderID] = peer
			}

			err := peer.AddCandidates(event.Session, event.Candidates)
			if err != nil {
				panic(err)
			}
		default:
			fmt.Printf("Unknown event type: %s\n", event)
		}
	}
}
