package handler

import (
	"github.com/corvych/nit/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ProxyHandler struct {
	proxyService service.ProxyService
}

func NewProxyHandler(ps service.ProxyService) *ProxyHandler {
	return &ProxyHandler{proxyService: ps}
}

func (h *ProxyHandler) AddProxy(c fiber.Ctx) error {
	var req service.AddProxyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name == "" || req.URL == "" || req.APIKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, url, and api_key are required"})
	}

	res, err := h.proxyService.AddProxy(c, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *ProxyHandler) ListAllProxies(c fiber.Ctx) error {
	res, err := h.proxyService.ListAllProxies(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}

func (h *ProxyHandler) DeleteProxy(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid proxy ID format"})
	}

	if err := h.proxyService.DeleteProxy(c, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Trusted proxy deleted successfully"})
}

func (h *ProxyHandler) ListActiveProxies(c fiber.Ctx) error {
	res, err := h.proxyService.ListActiveProxies(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}
