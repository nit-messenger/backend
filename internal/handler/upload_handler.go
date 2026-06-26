package handler

import (
	"log"
	"net/http"
	"os"

	"github.com/corvych/nit/internal/config"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

type UploadHandler struct {
	TusHandler *tusd.Handler
}

func NewUploadHandler(cfg *config.Config) *UploadHandler {
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(cfg.MediaStoragePath, 0755); err != nil {
		log.Fatalf("failed to create upload directory %s: %v", cfg.MediaStoragePath, err)
	}

	// Create store and locker
	store := filestore.New(cfg.MediaStoragePath)
	locker := filelocker.New(cfg.MediaStoragePath)

	// Compose storage components
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	// Create tusd handler
	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:      "/api/uploads/",
		StoreComposer: composer,
		MaxSize:       cfg.MaxUploadBytes,
	})
	if err != nil {
		log.Fatalf("unable to create tusd handler: %s", err)
	}

	return &UploadHandler{TusHandler: handler}
}

func (h *UploadHandler) HandleUpgrade(c fiber.Ctx) error {
	// Wrap tusd handler via Fiber HTTP handler adaptor
	// Strip prefix so tusd gets clean paths relative to BasePath
	netHandler := http.StripPrefix("/api/uploads/", h.TusHandler)
	return adaptor.HTTPHandler(netHandler)(c)
}
