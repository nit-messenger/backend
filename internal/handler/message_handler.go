package handler

import (
	"errors"
	"strconv"

	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type MessageHandler struct {
	messageService service.MessageService
}

func NewMessageHandler(ms service.MessageService) *MessageHandler {
	return &MessageHandler{messageService: ms}
}

func (h *MessageHandler) SendMessage(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	conversationID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID format",
		})
	}

	var req service.SendMessageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	req.ConversationID = conversationID
	if req.Type == "" {
		req.Type = "text"
	}

	res, err := h.messageService.SendMessage(c, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Note: Later we will hook WebSocket broadcast here. For now we return JSON response.
	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *MessageHandler) ListMessages(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	conversationID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID format",
		})
	}

	// Parse limit (query parameter)
	limitStr := c.Query("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	// Parse optional before message ID (query parameter)
	var beforeID *uuid.UUID
	beforeIDStr := c.Query("before")
	if beforeIDStr != "" {
		bID, err := uuid.Parse(beforeIDStr)
		if err == nil {
			beforeID = &bID
		}
	}

	res, err := h.messageService.ListMessages(c, userID, conversationID, limit, beforeID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}

func (h *MessageHandler) EditMessage(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	messageID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid message ID format",
		})
	}

	type editReq struct {
		Content string `json:"content"`
	}

	var req editReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	res, err := h.messageService.EditMessage(c, userID, messageID, req.Content)
	if err != nil {
		if errors.Is(err, service.ErrNotMessageSender) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}

func (h *MessageHandler) DeleteMessage(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	messageID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid message ID format",
		})
	}

	err = h.messageService.DeleteMessage(c, userID, messageID)
	if err != nil {
		if errors.Is(err, service.ErrNotMessageSender) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Message deleted successfully"})
}

func (h *MessageHandler) MarkAsRead(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	conversationID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID format",
		})
	}

	err = h.messageService.MarkConversationAsRead(c, userID, conversationID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Conversation marked as read"})
}
