package handler

import (
	"errors"

	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ConversationHandler struct {
	conversationService service.ConversationService
}

func NewConversationHandler(cs service.ConversationService) *ConversationHandler {
	return &ConversationHandler{conversationService: cs}
}

func (h *ConversationHandler) CreateConversation(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	var req service.CreateConversationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.FamilyID == uuid.Nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "family_id is required",
		})
	}

	if req.Type == "" {
		req.Type = "direct"
	}

	res, err := h.conversationService.CreateConversation(c, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *ConversationHandler) GetConversation(c fiber.Ctx) error {
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

	res, err := h.conversationService.GetConversation(c, userID, conversationID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Conversation not found"})
	}

	return c.JSON(res)
}

func (h *ConversationHandler) ListConversations(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	res, err := h.conversationService.ListConversations(c, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(res)
}
