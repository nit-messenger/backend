package handler

import (
	"errors"

	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(as service.AuthService) *AuthHandler {
	return &AuthHandler{authService: as}
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req service.RegisterRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" || req.DisplayName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, password, and display name are required",
		})
	}

	res, err := h.authService.Register(c, req)
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, service.ErrInvalidInvite) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req service.LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	res, err := h.authService.Login(c, req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCreds) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}

func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	type refreshReq struct {
		RefreshToken string `json:"refresh_token"`
	}

	var req refreshReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Refresh token is required",
		})
	}

	res, err := h.authService.Refresh(c, req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}

func (h *AuthHandler) Logout(c fiber.Ctx) error {
	type logoutReq struct {
		RefreshToken string `json:"refresh_token"`
	}

	var req logoutReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Refresh token is required",
		})
	}

	if err := h.authService.Logout(c, req.RefreshToken); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Logged out successfully"})
}

func (h *AuthHandler) Activate(c fiber.Ctx) error {
	var req service.ActivateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Username == "" || req.DisplayName == "" || req.Password == "" || req.FamilyName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username, display_name, password, and family_name are required"})
	}

	res, err := h.authService.Activate(c, req)
	if err != nil {
		if errors.Is(err, service.ErrAlreadyActivated) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *AuthHandler) CheckActivated(c fiber.Ctx) error {
	activated, err := h.authService.IsActivated(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"activated": activated})
}
