package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/corvych/nit/internal/webrtc"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/google/uuid"
	pion "github.com/pion/webrtc/v4"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512 KB
)

type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	UserID uuid.UUID
	Send   chan []byte
	SFU    *webrtc.SFUManager
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS Connection error: %v", err)
			}
			break
		}

		// Handle client event
		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("WS Unmarshal error: %v", err)
			continue
		}

		c.handleIncomingEvent(event)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleIncomingEvent(event Event) {
	switch event.Type {
	case "typing":
		var payload TypingPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		// Echo typing status back to client or relay it (handled by handlers/services)
	case "call_join":
		var payload CallJoinPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			log.Printf("Error unmarshaling call_join payload: %v", err)
			return
		}
		c.handleCallJoin(payload)
	case "call_leave":
		type leavePayload struct {
			ConversationID string `json:"conversation_id"`
		}
		var payload leavePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		c.handleCallLeave(payload.ConversationID)
	case "call_candidate":
		var payload CallCandidatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		c.handleCallCandidate(payload)
	default:
		log.Printf("Unhandled WS event type: %s", event.Type)
	}
}

func (c *Client) handleCallJoin(payload CallJoinPayload) {
	convID, err := uuid.Parse(payload.ConversationID)
	if err != nil {
		log.Printf("Invalid conversation ID in call_join: %s", payload.ConversationID)
		return
	}

	// 1. Create Peer Connection
	pc, err := c.SFU.CreatePeerConnection()
	if err != nil {
		log.Printf("Failed to create peer connection: %v", err)
		return
	}

	// Allow peer connection to receive audio/video
	if _, err = pc.AddTransceiverFromKind(pion.RTPCodecTypeAudio, pion.RTPTransceiverInit{Direction: pion.RTPTransceiverDirectionRecvonly}); err != nil {
		log.Printf("Failed to add audio transceiver: %v", err)
		return
	}
	if _, err = pc.AddTransceiverFromKind(pion.RTPCodecTypeVideo, pion.RTPTransceiverInit{Direction: pion.RTPTransceiverDirectionRecvonly}); err != nil {
		log.Printf("Failed to add video transceiver: %v", err)
		return
	}

	// 2. On new track published, add to room
	pc.OnTrack(func(remoteTrack *pion.TrackRemote, receiver *pion.RTPReceiver) {
		room := c.SFU.GetOrCreateRoom(convID)
		room.Publish(c.UserID, remoteTrack, receiver)
	})

	// 3. Trickle ICE Candidates
	pc.OnICECandidate(func(candidate *pion.ICECandidate) {
		if candidate == nil {
			return
		}
		candidateBytes, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			return
		}

		c.sendEvent("call_candidate", CallCandidatePayload{
			ConversationID: payload.ConversationID,
			Candidate:      string(candidateBytes),
		})
	})

	// 4. Set Remote Description (SDP Offer)
	offer := pion.SessionDescription{
		Type: pion.SDPTypeOffer,
		SDP:  payload.SDP,
	}

	if err = pc.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		return
	}

	// 5. Create SDP Answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create SDP answer: %v", err)
		return
	}

	// 6. Set Local Description
	if err = pc.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}

	// 7. Join SFU room
	room := c.SFU.GetOrCreateRoom(convID)
	_, err = room.Join(c.UserID, pc)
	if err != nil {
		log.Printf("Failed to join SFU Room: %v", err)
		return
	}

	// 8. Return Answer
	c.sendEvent("call_answer", CallAnswerPayload{
		ConversationID: payload.ConversationID,
		SDP:            answer.SDP,
	})
}

func (c *Client) handleCallLeave(convIDStr string) {
	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		return
	}

	room := c.SFU.GetOrCreateRoom(convID)
	room.Leave(c.UserID)
}

func (c *Client) handleCallCandidate(payload CallCandidatePayload) {
	convID, err := uuid.Parse(payload.ConversationID)
	if err != nil {
		return
	}

	room := c.SFU.GetOrCreateRoom(convID)

	var init pion.ICECandidateInit
	if err := json.Unmarshal([]byte(payload.Candidate), &init); err != nil {
		log.Printf("Failed to unmarshal ICE candidate init: %v", err)
		return
	}

	if err := room.AddCandidate(c.UserID, init); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
	}
}

func (c *Client) sendEvent(eventType string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	event := Event{
		Type:    eventType,
		Payload: payloadBytes,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return
	}
	c.Send <- eventBytes
}
