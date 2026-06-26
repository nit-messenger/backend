package handler

import (
	"errors"

	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type FamilyHandler struct {
	familyService service.FamilyService
}

func NewFamilyHandler(fs service.FamilyService) *FamilyHandler {
	return &FamilyHandler{familyService: fs}
}

func (h *FamilyHandler) CreateFamily(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	var req service.CreateFamilyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Family name is required",
		})
	}

	res, err := h.familyService.CreateFamily(c, userID, req.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *FamilyHandler) GetFamilyDetails(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	familyID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid family ID format",
		})
	}

	res, err := h.familyService.GetFamilyDetails(c, userID, familyID)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(res)
}

func (h *FamilyHandler) GenerateInvite(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	idStr := c.Params("id")
	familyID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid family ID format",
		})
	}

	code, err := h.familyService.GenerateInviteCode(c, userID, familyID)
	if err != nil {
		if errors.Is(err, service.ErrNotFamilyAdmin) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"invite_code": code,
	})
}

func (h *FamilyHandler) JoinFamily(c fiber.Ctx) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return err
	}

	type joinReq struct {
		InviteCode string `json:"invite_code"`
	}

	var req joinReq
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.InviteCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invite code is required",
		})
	}

	res, err := h.familyService.JoinFamily(c, userID, req.InviteCode)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInvite) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, service.ErrAlreadyMember) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(res)
}
