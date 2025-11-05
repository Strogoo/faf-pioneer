package adapter

import (
	"context"
	"faf-pioneer/applog"
	"faf-pioneer/faf"
	"faf-pioneer/gpgnet"
	"faf-pioneer/icebreaker"
	"faf-pioneer/launcher"
	"faf-pioneer/util"
	"faf-pioneer/webrtc"
	"fmt"
	pionwebrtc "github.com/pion/webrtc/v4"
	"go.uber.org/zap"
	"strings"
	"time"
	"sync"
)

var PeerManager     *webrtc.PeerManager
var userNicknames   =make(map[string]string)
var mu              sync.Mutex

type Adapter struct {
	gpgNetFromGame      chan gpgnet.Message
	gpgNetToGame        chan gpgnet.Message
	gpgNetToFafClient   chan gpgnet.Message
	gpgNetFromFafClient chan gpgnet.Message
	gameDataToGame      chan *[]byte
	icebreakerClient    *icebreaker.Client
	ctx                 context.Context
	cancel              context.CancelFunc
	launcherInfo        *launcher.Info
}

func New(ctx context.Context, cancel context.CancelFunc, info *launcher.Info) *Adapter {
	instance := &Adapter{
		ctx:                 ctx,
		cancel:              cancel,
		launcherInfo:        info,
		gpgNetFromGame:      make(chan gpgnet.Message),
		gpgNetToGame:        make(chan gpgnet.Message),
		gpgNetToFafClient:   make(chan gpgnet.Message),
		gpgNetFromFafClient: make(chan gpgnet.Message),
		gameDataToGame:      make(chan *[]byte),
		icebreakerClient:    icebreaker.NewClient(ctx, info.ApiRoot, info.GameId, info.AccessToken),
	}

	return instance
}

func (a *Adapter) WriteLogEntryToRemote(entries []*applog.LogEntry) error {
	return a.icebreakerClient.WriteLogEntryToRemote(entries)
}

func (a *Adapter) Start() error {
	// Gather ICE servers and listen for WebRTC events.
	sessionGameResponse, err := a.icebreakerClient.GetGameSession()
	if err != nil {
		return fmt.Errorf("could not query turn servers: %v", err)
	}

	// Start listening for ICE-Breaker events (SSE - server side event),
	// if listen are failed to connect or dropped connection after,
	// we should retry subscribing to "lobby" events.
	iceBreakerEventChannel := make(chan icebreaker.EventMessage)
	go func() {
		backoff := time.Second
		for {
			if err = a.icebreakerClient.Listen(iceBreakerEventChannel); err != nil {
				applog.Error("Could not start listening ICE-Breaker API (server-side) events", zap.Error(err))
			}
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(backoff):
				// Better don't increase delay and just use fixed 1 sec value
				// Might be critical in some situations when you want to restore conn as fast as possible
				if backoff < 0*time.Second {
					backoff *= 2
				}
			}
		}
	}()


	turnServer := make([]pionwebrtc.ICEServer, len(sessionGameResponse.Servers))
	for i, server := range sessionGameResponse.Servers {
		// Hardcoded until we remove FAF's cotrun from the list
		if len(server.Urls) > 0 {
			if strings.Contains(server.Urls[0], "139.162.142.250") {
				continue
			}
		}

		turnServer[i] = pionwebrtc.ICEServer{
			Username:       server.Username,
			Credential:     server.Credential,
			CredentialType: pionwebrtc.ICECredentialTypePassword,
			URLs:           make([]string, len(server.Urls)),
		}

		for j, url := range server.Urls {
			// for Java being Java reasons we unfortunately raped the URLs and need to convert it back.
			turnServer[i].URLs[j] = strings.ReplaceAll(url, "://", ":")
		}
	}

	// Debug obtained/available ICE servers.
	for _, server := range sessionGameResponse.Servers {
		applog.Debug("Stun/turn server", zap.Strings("urls", server.Urls))
	}

	if len(sessionGameResponse.Servers) == 0 {
		applog.Error("No stun/turn servers available, potential server misconfiguration")
	}

	// Lookup for free UDP port that we can start using for game UDP connections and start `util.GameUDPProxy`.
	gameUdpPort, err := util.GetFreeUdpPort()
	if err != nil {
		return fmt.Errorf("failed to find free udp peer port: %v", err)
	}
	applog.Debug("Selected UDP game port", zap.Uint("gamePort", gameUdpPort))

	// Create new WebRTC peer manager that would manage connections to other players (peers)
	// when we receive events from ICE-Breaker.
	peerManager := webrtc.NewPeerManager(
		a.ctx,
		a.icebreakerClient,
		a.launcherInfo,
		sessionGameResponse,
		gameUdpPort,
		turnServer,
		iceBreakerEventChannel,
		a.gpgNetToGame,
	)

	PeerManager = peerManager

	if peerManager.IsTurnRelayForced() {
		applog.Debug("Forcing TURN relay on")
	}

	// Initialize GPG-Net control plane server (connects to FAF.exe) and client (connects to FAF-Client).
	gpgNetServer := faf.NewGpgNetServer(a.ctx, a.cancel, peerManager, a.launcherInfo.GpgNetPort)
	gpgNetClient := faf.NewGpgNetClient(a.ctx, a.launcherInfo.GpgNetClientPort)

	// Redirect messages from FAF.exe to FAF-Client
	go util.RedirectChannelWithContext(a.ctx, a.gpgNetFromGame, a.gpgNetToFafClient)
	// Redirect messages from FAF-Client to FAF.exe
	go func() {
		for {
			select {
			case msg, ok := <-a.gpgNetFromFafClient:
				if !ok {
					return
				}

				if baseMsg, isBase := msg.(*gpgnet.BaseMessage); isBase {
					parsedMsg, parseErr := baseMsg.TryParse()
					if parseErr == nil {
						// Save nicknames for UI	
						cmd := parsedMsg.GetCommand()
						if cmd == "JoinGame" || cmd == "ConnectToPeer"{
							nickname := ""
							playerId := ""
							for i, item := range parsedMsg.GetArgs() {
								switch v := item.(type) {
								case int32:
									if i == 2 {
										playerId = fmt.Sprintf("%d", v)
									}
								case string:
									if i == 1 {
										nickname = v
									}
								}
								if playerId != "" && nickname != "" {
									mu.Lock()
									userNicknames[playerId] = nickname
									mu.Unlock()
								}
							}
						}

						processed := gpgNetServer.ProcessMessage(parsedMsg)
						a.gpgNetToGame <- processed
						continue
					}
				}

				a.gpgNetToGame <- msg
			case <-a.ctx.Done():
				return
			}
		}
	}()

	// Start the GPG-Net control server that acts like a primary bridge between game and this network adapter.
	go func() {
		if err = gpgNetServer.Listen(a.gpgNetFromGame, a.gpgNetToGame); err != nil {
			applog.Error("Failed to start listening GPG-Net control server connections", zap.Error(err))
		}
	}()

	// Start the GPG-Net client that will proxy data from game to FAF-Client.
	go func() {
		if err = gpgNetClient.Connect(a.gpgNetToFafClient, a.gpgNetFromFafClient); err != nil {
			applog.Error("Failed to start listening GPG-Net client proxy connections", zap.Error(err))
			a.cancel()
		}
	}()

	peerManager.Start()
	return nil
}

func GetPeerManager() *webrtc.PeerManager {
	return PeerManager
}

func GetNicknames() map[string]string {
	mu.Lock()
	nnames  := make(map[string]string, len(userNicknames))
	for k, v := range userNicknames {
        nnames[k] = v
    }
	mu.Unlock()
	return nnames
}