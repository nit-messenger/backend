package handler

import (
	"log"

	"github.com/corvych/nit/internal/model"
	"github.com/corvych/nit/internal/repository"
	"github.com/corvych/nit/internal/service"
	"github.com/corvych/nit/internal/ws"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type FederationHandler struct {
	fedService       service.FederationService
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	hub              *ws.Hub
}

func NewFederationHandler(
	fs service.FederationService,
	mr repository.MessageRepository,
	cr repository.ConversationRepository,
	hub *ws.Hub,
) *FederationHandler {
	return &FederationHandler{
		fedService:       fs,
		messageRepo:      mr,
		conversationRepo: cr,
		hub:              hub,
	}
}

// Node Management REST APIs

func (h *FederationHandler) AddNode(c fiber.Ctx) error {
	var req service.AddNodeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Domain == "" || req.BaseURL == "" || req.APIKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "domain, base_url, and api_key are required"})
	}

	res, err := h.fedService.AddNode(c, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *FederationHandler) ListNodes(c fiber.Ctx) error {
	res, err := h.fedService.ListNodes(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}

func (h *FederationHandler) DeleteNode(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid node ID format"})
	}

	if err := h.fedService.DeleteNode(c, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Trusted node deleted successfully"})
}

// Incoming Federation Protocol APIs

type IncomingMessagePayload struct {
	ConversationID string  `json:"conversation_id"`
	SenderName     string  `json:"sender_name"`
	Content        *string `json:"content"`
	Type           string  `json:"type"`
}

func (h *FederationHandler) ReceiveMessage(c fiber.Ctx) error {
	var payload IncomingMessagePayload
	if err := c.Bind().JSON(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	convID, err := uuid.Parse(payload.ConversationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid conversation ID"})
	}

	// 1. Verify if conversation exists locally
	conv, err := h.conversationRepo.GetByID(c, convID)
	if err != nil || conv == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Conversation not found on this node"})
	}

	// 2. Map system / federation user ID
	// For simplicity in P2P federation, we attribute federated messages to a placeholder ID
	// or use a system ID. Let's use the conversation's Creator ID or keep it as nil if nullable.
	// We'll use the creator ID for now.
	senderID := conv.CreatedBy

	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		SenderID:       senderID,
		Content:        payload.Content,
		Type:           payload.Type,
	}

	if err := h.messageRepo.Create(c, msg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 3. Broadcast to local participants online via WebSocket
	pIDs := make([]uuid.UUID, len(conv.Participants))
	for i, p := range conv.Participants {
		pIDs[i] = p.UserID
	}

	// Format response same as local message send
	senderName := payload.SenderName + " [Federated]"
	res := service.MessageResponse{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		SenderName:     senderName,
		Content:        msg.Content,
		Type:           msg.Type,
		CreatedAt:      msg.CreatedAt,
	}

	h.hub.BroadcastToUsers("new_message", res, pIDs)

	log.Printf("Federation: Successfully received and routed message for conversation: %s", convID)
	return c.SendStatus(fiber.StatusOK)
}

type IncomingCallSignalPayload struct {
	ConversationID string      `json:"conversation_id"`
	Type           string      `json:"type"`
	Payload        interface{} `json:"payload"`
}

func (h *FederationHandler) ReceiveCallSignal(c fiber.Ctx) error {
	var payload IncomingCallSignalPayload
	if err := c.Bind().JSON(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	convID, err := uuid.Parse(payload.ConversationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid conversation ID"})
	}

	// Verify conversation exists
	conv, err := h.conversationRepo.GetByID(c, convID)
	if err != nil || conv == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Conversation not found"})
	}

	// Forward signaling to all local participants via WS
	pIDs := make([]uuid.UUID, len(conv.Participants))
	for i, p := range conv.Participants {
		pIDs[i] = p.UserID
	}

	h.hub.BroadcastToUsers("federation_call_signal", payload, pIDs)

	log.Printf("Federation: Successfully relayed call signal type %s for conversation %s", payload.Type, convID)
	return c.SendStatus(fiber.StatusOK)
}
