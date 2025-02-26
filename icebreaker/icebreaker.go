package main

import (
	"flag"
	"fmt"
	"log"
	"resty.dev/v3"
	"strconv"
)

type SessionTokenRequest struct {
	GameId uint64 `json:"gameId"`
}

type SessionTokenResponse struct {
	Jwt string `json:"jwt"`
}

type SessionGameResponse struct {
	Id      string `json:"id"`
	Servers []struct {
		Id         string   `json:"id"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
		Urls       []string `json:"urls"`
	} `json:"servers"`
}

var client = resty.New()
var apiRoot *string

func main() {
	// Define flags without default values (not passing a value will cause an error)
	gameId := flag.Uint64("game-id", 0, "The ID of the game session")
	accessToken := flag.String("access-token", "", "The access token for authentication")
	apiRoot = flag.String("api-root", "https://api.faforever.com/ice", "The root uri of the icebreaker api")

	// Parse the command-line flags
	flag.Parse()

	// Validate that the required flags are provided
	if *gameId == 0 {
		log.Fatalf("Error: --game-id is required and must be a valid uint64.")
	}

	if *accessToken == "" {
		log.Fatalf("Error: --access-token is required and cannot be empty.")
	}

	sessionTokenResponse, err := getSessionToken(*accessToken, *gameId)

	if err != nil {
		log.Fatal("Could not query session token: ", err)
	}

	sessionGameResponse, err := getGameSession(sessionTokenResponse.Jwt, *gameId)

	if err != nil {
		log.Fatal("Could not query session game: ", err)
	}

	// Use the parsed data
	log.Printf("Parsed API Response: %+v\n", *sessionGameResponse)

	go listen(sessionTokenResponse.Jwt, *gameId)

	select {}
}

func getSessionToken(accessToken string, gameId uint64) (*SessionTokenResponse, error) {
	url := *apiRoot + "/session/token"

	requestData := SessionTokenRequest{
		GameId: gameId,
	}

	var result SessionTokenResponse

	// Make the POST request with JSON payload and Authorization header
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Content-Type", "application/json").
		SetBody(requestData).
		SetResult(&result).
		Post(url)

	if err != nil {
		return nil, fmt.Errorf("fetching session token failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("fetching session token failed: %v", resp.Status())
	}

	return &result, nil
}

func getGameSession(sessionToken string, gameId uint64) (*SessionGameResponse, error) {
	// Construct the URL with the gameId
	url := *apiRoot + "/session/game/" + strconv.FormatUint(gameId, 10)

	var result SessionGameResponse

	// Create a new HTTP request
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+sessionToken).
		SetResult(&result).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("fetching game session failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("fetching game session failed: %v", resp.Status())
	}

	return &result, nil
}

func sendEvent(sessionToken string, gameId uint64) error {
	url := *apiRoot + "/session/game/" + strconv.FormatUint(gameId, 10) + "/events"

	requestData := SessionTokenRequest{
		GameId: gameId,
	}

	// Make the POST request with JSON payload and Authorization header
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+sessionToken).
		SetHeader("Content-Type", "application/json").
		SetBody(requestData).
		Post(url)

	if err != nil {
		log.Fatalf("Posting session event failed: %v", err)
		return err
	}

	if resp.StatusCode() != 200 {
		log.Fatalf("Posting session event failed: %v", resp.Status())
		return err
	}

	return nil
}

func listen(sessionToken string, gameId uint64) {
	url := *apiRoot + "/session/game/" + strconv.FormatUint(gameId, 10) + "/events"

	eventSource := resty.NewEventSource().
		SetURL(url).
		SetHeader("Authorization", "Bearer "+sessionToken).
		OnMessage(func(message any) {
			restyEvent, ok := message.(*resty.Event)
			if !ok {
				log.Fatalf("Invalid event format")
				return
			}

			event, err := ParseEventMessage(restyEvent.Data)
			if err != nil {
				log.Fatalf("Error parsing event:", err)
				return
			}

			switch e := event.(type) {
			case *ConnectedMessage:
				log.Println("Handling a ConnectedMessage:", e)
			case *CandidatesMessage:
				log.Println("Handling a CandidatesMessage:", e)
			default:
				log.Println("Unknown event type")
			}
			log.Printf("Received event: %v\n\n", event)
		}, nil)

	err := eventSource.Get()

	if err != nil {
		log.Fatal("Could not query session game", err)
	}
}
