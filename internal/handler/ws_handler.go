package handler

import (
	"log"

	"github.com/corvych/nit/internal/webrtc"
	"github.com/corvych/nit/internal/ws"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// WSUpgradeHandler checks if the incoming request is a WebSocket upgrade request
func WSUpgradeHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			// Keep user ID in locals for the upgrade
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

// WSConnHandler manages the actual websocket connection lifecycle
func WSConnHandler(hub *ws.Hub, sfu *webrtc.SFUManager) fiber.Handler {
	return websocket.New(func(conn *websocket.Conn) {
		userIDVal := conn.Locals("userID")
		if userIDVal == nil {
			log.Println("WS Upgrade: missing userID in context")
			_ = conn.Close()
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			log.Println("WS Upgrade: invalid userID type in context")
			_ = conn.Close()
			return
		}

		client := &ws.Client{
			Hub:    hub,
			Conn:   conn,
			UserID: userID,
			Send:   make(chan []byte, 256),
			SFU:    sfu,
		}

		// Register client with hub
		hub.Register <- client

		// Start reader and writer pumps
		go client.WritePump()
		client.ReadPump()
	})
}
