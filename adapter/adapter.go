package adapter

import (
	"faf-pioneer/forgedalliance"
	"faf-pioneer/icebreaker"
	"faf-pioneer/util"
	"faf-pioneer/webrtc"
	pionwebrtc "github.com/pion/webrtc/v4"
	"github.com/samber/slog-multi"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
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
	logFile := openLogFileOrCrash(userId, gameId)
	defer logFile.Close()

	initLogger(userId, gameId, logFile)

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

func initLogger(userId uint, gameId uint64, logFile *os.File) {
	// Create console and file handlers
	consoleHandler := slog.NewJSONHandler(os.Stdout, nil)
	fileHandler := slog.NewJSONHandler(logFile, nil)

	// Use slog-multi to log to both handlers
	multiHandler := slogmulti.Fanout(consoleHandler, fileHandler)

	logger := slog.New(multiHandler).With(
		"userId", userId,
		"gameId", gameId,
	)
	slog.SetDefault(logger)
}

func openLogFileOrCrash(userId uint, gameId uint64) *os.File {
	// Generate a unique log filename using timestamp
	logFilename := "Game_" + strconv.FormatUint(gameId, 10) +
		"_User_" + strconv.Itoa(int(userId)) + "_" +
		time.Now().Format("2006-01-02_15-04-05") + ".log"

	// Open file for logging
	logFile, err := os.OpenFile(logFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("Failed to open log file", util.ErrorAttr(err))
		os.Exit(1)
	}
	return logFile
}
