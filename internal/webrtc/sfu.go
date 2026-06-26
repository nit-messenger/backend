package webrtc

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/corvych/nit/internal/config"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type Participant struct {
	UserID         uuid.UUID
	PeerConnection *webrtc.PeerConnection
	SendTrackMap   map[string]*webrtc.TrackLocalStaticRTP // track ID -> local static RTP track (being forwarded to this subscriber)
}

type TrackPublisher struct {
	UserID       uuid.UUID
	RemoteTrack  *webrtc.TrackRemote
	LocalTrack   *webrtc.TrackLocalStaticRTP
	StreamID     string
	TrackID      string
	CodecCapability webrtc.RTPCodecCapability
}

type CallRoom struct {
	ConversationID uuid.UUID
	Participants   map[uuid.UUID]*Participant
	Publishers     map[string]*TrackPublisher // track ID -> publisher
	mu             sync.RWMutex
}

type SFUManager struct {
	rooms   map[uuid.UUID]*CallRoom
	roomsMu sync.RWMutex
	config  webrtc.Configuration
}

func NewSFUManager() *SFUManager {
	// Standard WebRTC configuration
	// Note: In production, the user will configure the TURN/STUN settings.
	// We'll read these from the config.Config later or use default STUN
	cfg := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	return &SFUManager{
		rooms:  make(map[uuid.UUID]*CallRoom),
		config: cfg,
	}
}

func (s *SFUManager) GetOrCreateRoom(conversationID uuid.UUID) *CallRoom {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	room, exists := s.rooms[conversationID]
	if !exists {
		room = &CallRoom{
			ConversationID: conversationID,
			Participants:   make(map[uuid.UUID]*Participant),
			Publishers:     make(map[string]*TrackPublisher),
		}
		s.rooms[conversationID] = room
		log.Printf("SFU Room created for conversation: %s", conversationID)
	}
	return room
}

func (s *SFUManager) RemoveRoom(conversationID uuid.UUID) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	if _, exists := s.rooms[conversationID]; exists {
		delete(s.rooms, conversationID)
		log.Printf("SFU Room removed: %s", conversationID)
	}
}

// JoinRoom registers a participant's PeerConnection to a CallRoom and handles track forwarding
func (r *CallRoom) Join(userID uuid.UUID, pc *webrtc.PeerConnection) (*Participant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If participant already exists, close their previous PC
	if existing, ok := r.Participants[userID]; ok {
		_ = existing.PeerConnection.Close()
		delete(r.Participants, userID)
	}

	participant := &Participant{
		UserID:         userID,
		PeerConnection: pc,
		SendTrackMap:   make(map[string]*webrtc.TrackLocalStaticRTP),
	}
	r.Participants[userID] = participant

	// Subscribe this new participant to all existing publishers in the room
	for _, pub := range r.Publishers {
		if pub.UserID == userID {
			continue // Don't subscribe to yourself
		}
		rtpSender, err := pc.AddTrack(pub.LocalTrack)
		if err != nil {
			log.Printf("Failed to subscribe participant %s to track %s: %v", userID, pub.TrackID, err)
			continue
		}

		// Handle RTCP PLI requests to ask publisher for keyframe
		go func(sender *webrtc.RTPSender, pubUserID uuid.UUID, trackID string) {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}(rtpSender, pub.UserID, pub.TrackID)
	}

	// Trigger renegotiation on client side if necessary
	log.Printf("User %s joined SFU Call Room %s", userID, r.ConversationID)
	return participant, nil
}

// PublishTrack handles a new incoming media track from a participant, creates a local broadcast copy,
// and routes it to all other participants in the room.
func (r *CallRoom) Publish(userID uuid.UUID, remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create local track static RTP to forward packets to subscribers
	localTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, remoteTrack.ID(), remoteTrack.StreamID())
	if err != nil {
		log.Printf("Failed to create local track static RTP: %v", err)
		return
	}

	publisher := &TrackPublisher{
		UserID:          userID,
		RemoteTrack:     remoteTrack,
		LocalTrack:      localTrack,
		StreamID:        remoteTrack.StreamID(),
		TrackID:         remoteTrack.ID(),
		CodecCapability: remoteTrack.Codec().RTPCodecCapability,
	}
	r.Publishers[remoteTrack.ID()] = publisher

	// Subscribe all existing participants to this track (except the publisher)
	for pID, p := range r.Participants {
		if pID == userID {
			continue
		}
		rtpSender, err := p.PeerConnection.AddTrack(localTrack)
		if err != nil {
			log.Printf("Failed to subscribe existing participant %s to new track %s: %v", pID, remoteTrack.ID(), err)
			continue
		}

		// Handle RTCP PLI requests
		go func(sender *webrtc.RTPSender) {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}(rtpSender)
	}

	// Read RTP packets from remoteTrack and write them to localTrack
	go func() {
		defer r.Unpublish(remoteTrack.ID())

		rtpBuf := make([]byte, 1500)
		for {
			n, _, rtpErr := remoteTrack.Read(rtpBuf)
			if rtpErr != nil {
				return
			}

			if _, err = localTrack.Write(rtpBuf[:n]); err != nil {
				return
			}
		}
	}()

	log.Printf("Track %s published by user %s in Room %s", remoteTrack.ID(), userID, r.ConversationID)
}

// Unpublish removes a published track from the room
func (r *CallRoom) Unpublish(trackID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.Publishers[trackID]; exists {
		delete(r.Publishers, trackID)
		log.Printf("Track %s unpublished", trackID)
	}
}

// Leave removes a participant from the room and stops all their publishers/subscriptions
func (r *CallRoom) Leave(userID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	participant, exists := r.Participants[userID]
	if !exists {
		return
	}

	// Close peer connection
	_ = participant.PeerConnection.Close()
	delete(r.Participants, userID)

	// Remove all publishers belonging to this user
	for trackID, pub := range r.Publishers {
		if pub.UserID == userID {
			delete(r.Publishers, trackID)
			log.Printf("Publisher track %s removed because user %s left", trackID, userID)
		}
	}

	log.Printf("User %s left SFU Call Room %s", userID, r.ConversationID)
}

func (r *CallRoom) GetParticipants() []uuid.UUID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]uuid.UUID, 0, len(r.Participants))
	for id := range r.Participants {
		ids = append(ids, id)
	}
	return ids
}

// AddCandidate safely adds a trickle ICE candidate to a participant's PeerConnection
func (r *CallRoom) AddCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit) error {
	r.mu.RLock()
	participant, exists := r.Participants[userID]
	r.mu.RUnlock()

	if !exists {
		return errors.New("participant not found in call room")
	}

	return participant.PeerConnection.AddICECandidate(candidate)
}

// CreatePeerConnection helper to build peer connection with ICEServers
func (s *SFUManager) CreatePeerConnection() (*webrtc.PeerConnection, error) {
	api := webrtc.NewAPI()
	return api.NewPeerConnection(s.config)
}

// SetICEServers updates ICEServers list dynamically (e.g. from TURN settings)
func (s *SFUManager) SetICEServers(servers []webrtc.ICEServer) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()
	s.config.ICEServers = servers
}

// ConfigureSFUManager instantiates the SFU manager config with TURN/STUN settings from config.Config
func ConfigureSFUManager(cfg *config.Config) *SFUManager {
	sfu := NewSFUManager()

	iceServers := []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	}

	if cfg.TURNServerAddr != "" {
		turnServer := webrtc.ICEServer{
			URLs:           []string{fmt.Sprintf("turn:%s", cfg.TURNServerAddr)},
			Username:       cfg.TURNUsername,
			Credential:     cfg.TURNPassword,
			CredentialType: webrtc.ICECredentialTypePassword,
		}
		iceServers = append(iceServers, turnServer)
		log.Printf("Configured SFU manager with TURN Server: %s", cfg.TURNServerAddr)
	}

	sfu.SetICEServers(iceServers)
	return sfu
}
