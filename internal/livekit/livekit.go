package livekit

import (
	"time"

	"github.com/livekit/protocol/auth"
)

// GenerateToken creates a JWT access token for a LiveKit client to join a specific room.
func GenerateToken(apiKey, apiSecret, roomName, identity string) (string, error) {
	at := auth.NewAccessToken(apiKey, apiSecret)
	
	// Set participant details and token lifespan (1 hour)
	at.SetIdentity(identity).SetValidFor(time.Hour)
	
	// Configure room join permissions
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.SetVideoGrant(grant)
	
	return at.ToJWT()
}
