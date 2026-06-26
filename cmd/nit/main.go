package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/corvych/nit/internal/config"
	"github.com/corvych/nit/internal/database"
	"github.com/corvych/nit/internal/handler"
	"github.com/corvych/nit/internal/middleware"
	"github.com/corvych/nit/internal/repository"
	"github.com/corvych/nit/internal/retention"
	"github.com/corvych/nit/internal/service"
	"github.com/corvych/nit/internal/ws"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

func main() {
	// 1. Load config
	cfg := config.LoadConfig()

	// 2. Initialize Database
	db := database.InitDB(cfg)

	// 3. Initialize WebSocket Hub, WebRTC SFU, and Retention Worker
	hub := ws.NewHub()
	go hub.Run()


	// Start background retention worker (runs every 1 hour)
	retentionCtx, cancelRetention := context.WithCancel(context.Background())
	defer cancelRetention()
	retWorker := retention.NewRetentionWorker(db, 1*time.Hour)
	retWorker.Start(retentionCtx)

	// Start background proxy registration worker
	proxyCtx, cancelProxy := context.WithCancel(context.Background())
	defer cancelProxy()

	// 4. Initialize Repositories
	userRepo := repository.NewUserRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	conversationRepo := repository.NewConversationRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	callRepo := repository.NewCallRepository(db)
	pushRepo := repository.NewPushSubscriptionRepository(db)
	nodeRepo := repository.NewTrustedNodeRepository(db)
	proxyRepo := repository.NewTrustedProxyRepository(db)

	// 5. Initialize Services
	authService := service.NewAuthService(userRepo, familyRepo, tokenRepo, cfg)
	userService := service.NewUserService(userRepo)
	familyService := service.NewFamilyService(familyRepo)
	conversationService := service.NewConversationService(conversationRepo, familyRepo)
	pushService := service.NewPushService(pushRepo, cfg)
	fedService := service.NewFederationService(nodeRepo)
	messageService := service.NewMessageService(messageRepo, conversationRepo, hub, pushService, fedService)
	callService := service.NewCallService(callRepo, conversationRepo, hub, cfg)
	proxyService := service.NewProxyService(proxyRepo, cfg)

	// Start proxy service worker
	proxyService.StartRegistrationWorker(proxyCtx)

	// 6. Initialize Handlers
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	familyHandler := handler.NewFamilyHandler(familyService)
	conversationHandler := handler.NewConversationHandler(conversationService)
	messageHandler := handler.NewMessageHandler(messageService)
	uploadHandler := handler.NewUploadHandler(cfg)
	callHandler := handler.NewCallHandler(callService)
	pushHandler := handler.NewPushHandler(pushService)
	fedHandler := handler.NewFederationHandler(fedService, messageRepo, conversationRepo, hub)
	proxyHandler := handler.NewProxyHandler(proxyService)

	// 7. Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Nit Backend v3",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// Add CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "Tus-Resumable", "Upload-Length", "Upload-Metadata", "Upload-Offset", "X-Requested-With", "X-Federation-Key", "X-Proxy-Key"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
	}))

	// Health check route
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Setup Middlewares
	authMiddleware := middleware.NewAuthMiddleware(cfg)
	fedAuthMiddleware := middleware.NewFederationAuth(nodeRepo)

	// Register WebSocket Route (Public check → Auth Middleware → Conn Handler)
	app.Get("/ws", handler.WSUpgradeHandler(), authMiddleware, handler.WSConnHandler(hub))

	// Register TUS Upload Route
	app.All("/api/uploads/*", authMiddleware, uploadHandler.HandleUpgrade)

	// 8. Register API Routes
	api := app.Group("/api")

	// Public Auth routes
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Post("/logout", authHandler.Logout)
	auth.Post("/activate", authHandler.Activate)
	auth.Get("/activated", authHandler.CheckActivated)

	// Public proxies list route
	api.Get("/proxies", proxyHandler.ListActiveProxies)

	// Private User routes
	users := api.Group("/users", authMiddleware)
	users.Get("/me", userHandler.GetMe)
	users.Patch("/me", userHandler.UpdateMe)
	users.Get("/:id", userHandler.GetByID)

	// Private Family routes
	families := api.Group("/families", authMiddleware)
	families.Post("/", familyHandler.CreateFamily)
	families.Get("/:id", familyHandler.GetFamilyDetails)
	families.Post("/:id/invite", familyHandler.GenerateInvite)
	families.Post("/join", familyHandler.JoinFamily)

	// Private Conversation routes
	conversations := api.Group("/conversations", authMiddleware)
	conversations.Post("/", conversationHandler.CreateConversation)
	conversations.Get("/", conversationHandler.ListConversations)
	conversations.Get("/:id", conversationHandler.GetConversation)

	// Private Message routes
	conversations.Post("/:id/messages", messageHandler.SendMessage)
	conversations.Get("/:id/messages", messageHandler.ListMessages)
	conversations.Post("/:id/read", messageHandler.MarkAsRead)

	messages := api.Group("/messages", authMiddleware)
	messages.Patch("/:id", messageHandler.EditMessage)
	messages.Delete("/:id", messageHandler.DeleteMessage)

	// Private Call routes
	conversations.Post("/:id/calls", callHandler.StartCall)
	conversations.Get("/:id/calls", callHandler.GetCallHistory)

	calls := api.Group("/calls", authMiddleware)
	calls.Post("/:id/join", callHandler.JoinCall)
	calls.Post("/:id/leave", callHandler.LeaveCall)

	// Private Push routes
	push := api.Group("/push", authMiddleware)
	push.Post("/subscribe", pushHandler.Subscribe)
	push.Post("/unsubscribe", pushHandler.Unsubscribe)

	// Private Federation Node Management routes
	fedNodes := api.Group("/federation/nodes", authMiddleware)
	fedNodes.Post("/", fedHandler.AddNode)
	fedNodes.Get("/", fedHandler.ListNodes)
	fedNodes.Delete("/:id", fedHandler.DeleteNode)

	// Private Trusted Proxies Management routes
	adminProxies := api.Group("/admin/proxies", authMiddleware)
	adminProxies.Post("/", proxyHandler.AddProxy)
	adminProxies.Get("/", proxyHandler.ListAllProxies)
	adminProxies.Delete("/:id", proxyHandler.DeleteProxy)

	// Incoming federation routes
	fedIncoming := api.Group("/federation", fedAuthMiddleware)
	fedIncoming.Post("/messages", fedHandler.ReceiveMessage)
	fedIncoming.Post("/calls", fedHandler.ReceiveCallSignal)

	// 9. Start server in a goroutine
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// 10. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop background retention and proxy workers
	cancelRetention()
	cancelProxy()

	// Create a context with timeout for shutdown
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}
