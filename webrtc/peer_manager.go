package webrtc

import (
	"faf-pioneer/icebreaker"
	"fmt"
	pionwebrtc "github.com/pion/webrtc/v4"
	"log"
)

type PeerManager struct {
	userId           uint
	gameId           uint64
	peers            map[uint]*Peer
	icebreakerClient *icebreaker.Client
	icebreakerEvents <-chan icebreaker.EventMessage
	turnServer       []pionwebrtc.ICEServer
	gameUdpPort      uint
	nextPeerUdpPort  uint
}

func NewPeerManager(
	icebreakerClient *icebreaker.Client,
	userId uint,
	gameId uint64,
	gameUdpPort uint,
	basePeerUdpPort uint,
	turnServer []pionwebrtc.ICEServer,
	icebreakerEvents <-chan icebreaker.EventMessage,
) PeerManager {
	peerManager := PeerManager{
		userId:           userId,
		gameId:           gameId,
		peers:            make(map[uint]*Peer),
		icebreakerClient: icebreakerClient,
		icebreakerEvents: icebreakerEvents,
		turnServer:       turnServer,
		gameUdpPort:      gameUdpPort,
		nextPeerUdpPort:  basePeerUdpPort,
	}

	return peerManager
}

func (p *PeerManager) Start() {
	for msg := range p.icebreakerEvents {
		switch event := msg.(type) {
		case *icebreaker.ConnectedMessage:
			log.Printf("Connecting to peer: %s\n", event)

			peer, err := CreatePeer(true, event.SenderID, p.turnServer, p.nextPeerUdpPort, p.gameUdpPort, p.onCandidatesGathered(event.SenderID))

			if err != nil {
				panic(err)
			}

			p.peers[event.SenderID] = peer
			p.nextPeerUdpPort++
		case *icebreaker.CandidatesMessage:
			fmt.Printf("Received CandidatesMessage: %s\n", event)
			peer := p.peers[event.SenderID]

			if peer == nil {
				newPeer, err := CreatePeer(false, event.SenderID, p.turnServer, p.nextPeerUdpPort, p.gameUdpPort, p.onCandidatesGathered(event.SenderID))
				if err != nil {
					panic(err)
				}

				peer = newPeer
				p.peers[event.SenderID] = peer
				p.nextPeerUdpPort++
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

func (p *PeerManager) onCandidatesGathered(remotePeer uint) func(*pionwebrtc.SessionDescription, []pionwebrtc.ICECandidate) {
	return func(description *pionwebrtc.SessionDescription, candidates []pionwebrtc.ICECandidate) {
		err := p.icebreakerClient.SendEvent(
			icebreaker.CandidatesMessage{
				BaseEvent: icebreaker.BaseEvent{
					EventType:   "candidates",
					GameID:      p.gameId,
					SenderID:    p.userId,
					RecipientID: &remotePeer,
				},
				Session:    description,
				Candidates: candidates,
			})

		if err != nil {
			log.Printf("Failed to send candidates: %s\n", err)
		}
	}
}
