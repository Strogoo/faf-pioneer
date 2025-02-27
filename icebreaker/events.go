package icebreaker

import (
	"encoding/json"
	"fmt"
)

type EventMessage interface {
	GetSenderId() uint32
	GetRecipientId() *uint32
}

type BaseEvent struct {
	EventType   string  `json:"eventType"`
	GameID      uint64  `json:"gameId"`
	SenderID    uint32  `json:"senderId"`
	RecipientID *uint32 `json:"recipientId,omitempty"`
}

type ConnectedMessage struct {
	GameID      uint64  `json:"gameId"`
	SenderID    uint32  `json:"senderId"`
	RecipientID *uint32 `json:"recipientId,omitempty"`
}

func (e ConnectedMessage) String() string {
	return fmt.Sprintf("ConnectedMessage { GameId=%d, SenderId=%d, RecipientId=%v }", e.GameID, e.SenderID, e.RecipientID)
}
func (e ConnectedMessage) GetSenderId() uint32     { return e.SenderID }
func (e ConnectedMessage) GetRecipientId() *uint32 { return e.RecipientID }

type CandidatesMessage struct {
	GameID      uint64  `json:"gameId"`
	SenderID    uint32  `json:"senderId"`
	RecipientID *uint32 `json:"recipientId"`
	Candidates  []struct {
		Foundation string `json:"foundation"`
		Protocol   string `json:"protocol"`
		Priority   int    `json:"priority"`
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		Type       string `json:"type"`
		Generation int    `json:"generation"`
		ID         string `json:"id"`
		RelAddr    string `json:"relAddr"`
		RelPort    int    `json:"relPort"`
	} `json:"candidates"` // Replace with CandidateDescriptor struct if needed
}

func (e CandidatesMessage) String() string {
	return fmt.Sprintf("CandidatesMessage { GameId=%d, SenderId=%d, RecipientId=%v }", e.GameID, e.SenderID, e.RecipientID)
}
func (e CandidatesMessage) GetSenderId() uint32     { return e.SenderID }
func (e CandidatesMessage) GetRecipientId() *uint32 { return e.RecipientID }

func ParseEventMessage(message string) (EventMessage, error) {
	// First, decode into a generic map to extract eventType
	var data = []byte(message)
	var raw BaseEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Based on eventType, unmarshal into the correct struct
	switch raw.EventType {
	case "connected":
		var msg ConnectedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "candidates":
		var msg CandidatesMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unknown eventType: %s", raw.EventType)
	}
}
