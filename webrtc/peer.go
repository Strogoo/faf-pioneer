package webrtc

import (
	"fmt"
	"github.com/pion/randutil"
	"github.com/pion/webrtc/v4"
	"log"
	"sync"
	"time"
)

type Peer struct {
	Offerer              bool
	peerId               uint
	connection           *webrtc.PeerConnection
	gameDataChannel      *webrtc.DataChannel
	offer                *webrtc.SessionDescription
	answer               *webrtc.SessionDescription
	pendingCandidates    []*webrtc.ICECandidate
	candidatesMux        sync.Mutex
	onCandidatesGathered func(*webrtc.SessionDescription, []*webrtc.ICECandidate)
}

func (p *Peer) wrapError(format string, a ...any) error {
	return fmt.Errorf("[Peer %d] %s", p.peerId, fmt.Sprintf(format, a...))
}

func CreatePeer(
	offerer bool,
	peerId uint,
	iceServers []webrtc.ICEServer,
	onCandidatesGathered func(*webrtc.SessionDescription, []*webrtc.ICECandidate)) (*Peer, error) {
	var err error
	peer := Peer{
		Offerer:              offerer,
		peerId:               peerId,
		onCandidatesGathered: onCandidatesGathered,
	}

	connection, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})

	if err != nil {
		return nil, peer.wrapError("cannot create peer connection", err)
	}

	// default is ordered and announced, we don't need to pass options
	dataChannel, err := connection.CreateDataChannel("gameData", nil)
	if err != nil {
		return nil, peer.wrapError("cannot create data channel", err)
	}

	if offerer {
		// Sets the LocalDescription, and starts our UDP listeners
		// Note: this will start the gathering of ICE candidates
		offer, err := connection.CreateOffer(nil)
		if err != nil {
			panic(err)
		}

		peer.offer = &offer

		if err = connection.SetLocalDescription(offer); err != nil {
			panic(err)
		}
	}

	connection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			var sessionDescription *webrtc.SessionDescription

			if peer.Offerer {
				sessionDescription = peer.offer
			} else {
				sessionDescription = peer.answer
			}

			peer.onCandidatesGathered(sessionDescription, peer.pendingCandidates)
			return
		}

		peer.candidatesMux.Lock()
		defer peer.candidatesMux.Unlock()

		peer.pendingCandidates = append(peer.pendingCandidates, candidate)
	})

	connection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer Connection State has changed %s \n", state.String())
	})

	// Register data channel creation handling
	connection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())

		// Register channel opening handling
		dataChannel.OnOpen(func() {
			fmt.Printf(
				"Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n",
				dataChannel.Label(), dataChannel.ID(),
			)

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				message, sendTextErr := randutil.GenerateCryptoRandomString(
					15, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
				)
				if sendTextErr != nil {
					panic(sendTextErr)
				}

				// Send the message as text
				fmt.Printf("Sending '%s'\n", message)
				if sendTextErr = dataChannel.SendText(message); sendTextErr != nil {
					panic(sendTextErr)
				}
			}
		})

		// Register text message handling
		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
		})
	})

	peer.connection = connection
	peer.gameDataChannel = dataChannel

	return &peer, nil
}

func (p *Peer) AddCandidates(session *webrtc.SessionDescription, candidates []*webrtc.ICECandidate) error {
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

	if !p.Offerer {
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
	if err := p.connection.Close(); err != nil {
		return p.wrapError("cannot close peerConnection: %v\n", err)
	}

	return nil
}
