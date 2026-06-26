package handler

import (
	"errors"

	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type CallHandler struct {
	callService service.CallService
}

func NewCallHandler(cs service.CallService) *CallHandler {
	return &CallHandler{callService: cs}
}

func (h *CallHandler) StartCall(c fiber.Ctx) error {
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

	var req service.StartCallRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	req.ConversationID = conversationID
	if req.Type == "" {
		req.Type = "audio"
	}

	res, err := h.callService.StartCall(c, userID, req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *CallHandler) JoinCall(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	callID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid call ID format",
		})
	}

	res, err := h.callService.JoinCall(c, userID, callID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}

func (h *CallHandler) LeaveCall(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	callID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid call ID format",
		})
	}

	err = h.callService.LeaveCall(c, userID, callID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Left call successfully"})
}

func (h *CallHandler) GetCallHistory(c fiber.Ctx) error {
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

	res, err := h.callService.GetCallHistory(c, userID, conversationID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}
