package handler

import (
	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
)

type PushHandler struct {
	pushService service.PushService
}

func NewPushHandler(ps service.PushService) *PushHandler {
	return &PushHandler{pushService: ps}
}

func (h *PushHandler) Subscribe(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	var req service.SubscribeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Platform == "" || req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "platform and token are required fields",
		})
	}

	if err := h.pushService.Subscribe(c, userID, req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Subscribed to push notifications successfully"})
}

func (h *PushHandler) Unsubscribe(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	type unsubscribeRequest struct {
		Token string `json:"token"`
	}

	var req unsubscribeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "token is required",
		})
	}

	if err := h.pushService.Unsubscribe(c, userID, req.Token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Unsubscribed from push notifications successfully"})
}
