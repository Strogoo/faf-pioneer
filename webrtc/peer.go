package webrtc

import (
	"context"
	"encoding/json"
	"faf-pioneer/util"
	"fmt"
	"github.com/pion/webrtc/v4"
	"log/slog"
	"sync"
)

type PeerMeta interface {
	IsOfferer() bool
	PeerId() uint
}

type Peer struct {
	offerer              bool
	peerId               uint
	context              context.Context
	connection           *webrtc.PeerConnection
	gameDataChannel      *webrtc.DataChannel
	offer                *webrtc.SessionDescription
	answer               *webrtc.SessionDescription
	pendingCandidates    []webrtc.ICECandidate
	candidatesMux        sync.Mutex
	onCandidatesGathered func(*webrtc.SessionDescription, []webrtc.ICECandidate)
	gameToWebrtcChannel  chan []byte
	webrtcToGameChannel  chan []byte
	gameDataProxy        *util.GameUDPProxy
}

func (p *Peer) IsOfferer() bool {
	return p.offerer
}

func (p *Peer) PeerId() uint {
	return p.peerId
}

func (p *Peer) wrapError(format string, a ...any) error {
	return fmt.Errorf("[Peer %d] %s", p.peerId, fmt.Sprintf(format, a...))
}

func CreatePeer(
	offerer bool,
	peerId uint,
	iceServers []webrtc.ICEServer,
	gameToWebrtcPort uint,
	webrtcToGamePort uint,
	onCandidatesGathered func(*webrtc.SessionDescription, []webrtc.ICECandidate)) (*Peer, error) {
	var err error

	ctx := context.Background()
	ctx = context.WithValue(ctx, "remotePlayerId", peerId)
	ctx = context.WithValue(ctx, "localOfferer", offerer)

	gameToWebrtcChannel := make(chan []byte)
	webrtcToGameChannel := make(chan []byte)

	gameUdpProxy, err := util.NewGameUDPProxy(
		webrtcToGamePort, gameToWebrtcPort, gameToWebrtcChannel, webrtcToGameChannel,
	)
	if err != nil {
		return nil, err
	}

	peer := Peer{
		offerer:              offerer,
		peerId:               peerId,
		gameToWebrtcChannel:  gameToWebrtcChannel,
		webrtcToGameChannel:  webrtcToGameChannel,
		onCandidatesGathered: onCandidatesGathered,
		gameDataProxy:        gameUdpProxy,
	}

	connection, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
	if err != nil {
		return nil, peer.wrapError("cannot create peer connection", err)
	}

	connection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		peer.candidatesMux.Lock()
		defer peer.candidatesMux.Unlock()

		if candidate == nil {
			var sessionDescription *webrtc.SessionDescription

			if peer.offerer {
				sessionDescription = peer.offer
			} else {
				sessionDescription = peer.answer
			}

			peer.onCandidatesGathered(sessionDescription, peer.pendingCandidates)
			return
		}

		peer.pendingCandidates = append(peer.pendingCandidates, *candidate)
	})

	connection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.InfoContext(ctx, "Peer Connection State has changed", slog.String("state", state.String()))

		if state == webrtc.PeerConnectionStateConnected {
			var selectedCandidatePair webrtc.ICECandidatePairStats
			candidates := make(map[string]webrtc.ICECandidateStats)

			for _, s := range connection.GetStats() {
				switch stat := s.(type) {
				case webrtc.ICECandidateStats:
					candidates[stat.ID] = stat
				case webrtc.ICECandidatePairStats:
					if stat.State == webrtc.StatsICECandidatePairStateSucceeded {
						selectedCandidatePair = stat
					}
				default:
				}
			}

			localCandidateJson, err := json.Marshal(candidates[selectedCandidatePair.LocalCandidateID])
			if err != nil {
				slog.WarnContext(ctx, "Failed to serialize local candidate", util.ErrorAttr(err))
			} else {
				slog.InfoContext(ctx, "Local candidate", slog.String("candidate", string(localCandidateJson)))
			}

			remoteCandidateJson, err := json.Marshal(candidates[selectedCandidatePair.RemoteCandidateID])
			if err != nil {
				slog.WarnContext(ctx, "Failed to serialize remote candidate", util.ErrorAttr(err))
			} else {
				slog.InfoContext(ctx, "Remote candidate", slog.String("candidate", string(remoteCandidateJson)))
			}
		}
	})

	// Register data channel creation handling
	connection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		peer.gameDataChannel = dataChannel
		peer.RegisterDataChannel()
		dataChannel.Transport()
	})

	peer.connection = connection

	return &peer, nil
}

func (p *Peer) InitiateConnection() error {
	if p.offerer && p.connection.ICEConnectionState() == webrtc.ICEConnectionStateNew {
		slog.InfoContext(p.context, "Initiating connection")

		// default is ordered and announced, we don't need to pass options
		dataChannel, err := p.connection.CreateDataChannel("gameData", nil)
		if err != nil {
			return p.wrapError("cannot create data channel", err)
		}

		p.gameDataChannel = dataChannel
		p.RegisterDataChannel()

		// Sets the LocalDescription, and starts our UDP listeners
		// Note: this will start the gathering of ICE candidates
		offer, err := p.connection.CreateOffer(nil)
		if err != nil {
			return p.wrapError("cannot create offer", err)
		}

		p.offer = &offer

		if err = p.connection.SetLocalDescription(offer); err != nil {
			return p.wrapError("cannot set local description", err)
		}
	} else {
		slog.DebugContext(p.context, "Not initiating connection")
	}

	return nil
}

func (p *Peer) AddCandidates(session *webrtc.SessionDescription, candidates []webrtc.ICECandidate) error {
	p.answer = session

	err := p.connection.SetRemoteDescription(*session)
	if err != nil {
		panic(err)
	}

	for _, candidate := range candidates {
		err := p.connection.AddICECandidate(candidate.ToJSON())
		if err != nil {
			return p.wrapError("cannot add candidate to peer", err)
		}
	}

	if !p.offerer {
		answer, err := p.connection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		p.answer = &answer
		// Sets the LocalDescription, and starts our UDP listeners
		err = p.connection.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func (p *Peer) Close() error {
	p.gameDataProxy.Close()
	if err := p.connection.Close(); err != nil {
		return p.wrapError("cannot close peerConnection: %v\n", err)
	}

	return nil
}

func (p *Peer) RegisterDataChannel() {
	slog.InfoContext(p.context, "Registering data channel handlers",
		slog.String("label", p.gameDataChannel.Label()),
		//		slog.Any("id", *p.gameDataChannel.ID()),
	)

	// Register channel opening handling
	p.gameDataChannel.OnOpen(func() {
		slog.InfoContext(p.context, "Data channel opened",
			slog.String("label", p.gameDataChannel.Label()),
			slog.Any("id", *p.gameDataChannel.ID()),
		)

		go func() {
			for msg := range p.gameToWebrtcChannel {
				err := p.gameDataChannel.Send(msg)
				if err != nil {
					slog.ErrorContext(p.context, "Could not send data to WebRTC data channel", util.ErrorAttr(err))
				}
			}
		}()
	})

	// Register text message handling
	p.gameDataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		p.webrtcToGameChannel <- msg.Data
	})
}
