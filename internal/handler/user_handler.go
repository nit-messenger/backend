package handler

import (
	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(us service.UserService) *UserHandler {
	return &UserHandler{userService: us}
}

func (h *UserHandler) GetMe(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	user, err := h.userService.GetByID(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"avatar_url":   user.AvatarURL,
		"status":       user.Status,
		"server_domain":user.ServerDomain,
		"created_at":   user.CreatedAt,
	})
}

func (h *UserHandler) UpdateMe(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	type updateReq struct {
		DisplayName string  `json:"display_name"`
		Email       string  `json:"email"`
		AvatarURL   *string `json:"avatar_url"`
	}

	var req updateReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	user, err := h.userService.Update(c, userID, req.DisplayName, req.Email, req.AvatarURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"avatar_url":   user.AvatarURL,
		"status":       user.Status,
	})
}

func (h *UserHandler) GetByID(c fiber.Ctx) error {
	idStr := c.Params("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	user, err := h.userService.GetByID(c, targetID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"status":       user.Status,
		"server_domain":user.ServerDomain,
		"last_seen_at": user.LastSeenAt,
	})
}
