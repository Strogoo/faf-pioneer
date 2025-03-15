package adapter

import (
	"faf-pioneer/forgedalliance"
	"faf-pioneer/icebreaker"
	"faf-pioneer/util"
	"faf-pioneer/webrtc"
	pionwebrtc "github.com/pion/webrtc/v4"
	"log/slog"
	"os"
	"strings"
)

type GlobalChannels struct {
	gpgNetFromGame      chan *forgedalliance.GpgMessage
	gpgNetToGame        chan *forgedalliance.GpgMessage
	gpgNetToFafClient   chan *forgedalliance.GpgMessage
	gpgNetFromFafClient chan *forgedalliance.GpgMessage
	gameDataToGame      chan *[]byte
}

func Start(
	userId uint,
	gameId uint64,
	accessToken string,
	apiRoot string,
	gpgNetPort uint,
	gpgNetClientPort uint,
	gameUdpPort uint,
) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		"userId", userId,
		"gameId", gameId,
	)
	slog.SetDefault(logger)

	globalChannels := GlobalChannels{
		gpgNetFromGame:      make(chan *forgedalliance.GpgMessage),
		gpgNetToGame:        make(chan *forgedalliance.GpgMessage),
		gpgNetToFafClient:   make(chan *forgedalliance.GpgMessage),
		gpgNetFromFafClient: make(chan *forgedalliance.GpgMessage),
		gameDataToGame:      make(chan *[]byte),
	}

	// Wire GpgNetClient to GpgNetServer
	go func() {
		for msg := range globalChannels.gpgNetFromGame {
			globalChannels.gpgNetToFafClient <- msg
		}
	}()

	go func() {
		for msg := range globalChannels.gpgNetFromFafClient {
			globalChannels.gpgNetToGame <- msg
		}
	}()

	// Gather ICE servers and listen for WebRTC events
	icebreakerClient := icebreaker.NewClient(apiRoot, gameId, accessToken)
	sessionGameResponse, err := icebreakerClient.GetGameSession()

	if err != nil {
		slog.Error("Could not query turn servers", util.ErrorAttr(err))
		os.Exit(1)
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

	var peerUdpPort uint = 18000 // TODO: Pick a "free random one"?

	peerManager := webrtc.NewPeerManager(
		&icebreakerClient,
		userId,
		gameId,
		gameUdpPort,
		peerUdpPort,
		turnServer,
		channel,
	)

	peerManager.Start()

	// Start the Gpgnet Control Server
	gpgNetServer := forgedalliance.NewGpgNetServer(&peerManager, gpgNetPort)
	go gpgNetServer.Listen(globalChannels.gpgNetFromGame, globalChannels.gpgNetToGame)

	// Start the GpgNet client to proxy data to the FAF client
	gpgNetClient := forgedalliance.NewGpgNetClient(gpgNetClientPort)
	go gpgNetClient.Listen(globalChannels.gpgNetToFafClient, globalChannels.gpgNetFromFafClient)
}
