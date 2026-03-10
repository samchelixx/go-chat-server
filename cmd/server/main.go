// Command server is the entry point for the Go Chat Server.
//
// It wires together all application components:
//   - Loads configuration from the environment
//   - Opens the database connection
//   - Starts the WebSocket Hub goroutine
//   - Registers all Gin routes
//   - Serves the static frontend
//   - Shuts down gracefully on SIGINT / SIGTERM
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samdevgo/go-chat-server/internal/auth"
	"github.com/samdevgo/go-chat-server/internal/chat"
	"github.com/samdevgo/go-chat-server/internal/config"
	"github.com/samdevgo/go-chat-server/internal/db"
	"github.com/samdevgo/go-chat-server/internal/room"
)

func main() {
	// ── 1. Configuration ────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// ── 2. Database ──────────────────────────────────────────────────────────
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	// ── 3. Chat Hub ──────────────────────────────────────────────────────────
	// The Hub runs in its own goroutine and acts as the single owner of all
	// WebSocket state, eliminating the need for locks in the broadcast path.
	hub := chat.NewHub(database)
	go hub.Run()

	// ── 4. HTTP Router ───────────────────────────────────────────────────────
	// Use Release mode in production; debug mode adds verbose route logging.
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Serve the frontend from the /web directory.
	// In Docker the web/ folder is copied alongside the binary.
	router.Static("/web", "./web")
	router.StaticFile("/", "./web/index.html")
	router.StaticFile("/chat", "./web/chat.html")

	// ── REST API routes ───────────────────────────────────────────────────────

	// Auth — public, no JWT required.
	authHandler := auth.NewHandler(database, cfg.JWTSecret, cfg.JWTExpiresHours)
	apiAuth := router.Group("/api/auth")
	{
		apiAuth.POST("/register", authHandler.Register)
		apiAuth.POST("/login", authHandler.Login)
	}

	// Rooms — list is public; create requires authentication.
	roomHandler := room.NewHandler(database)
	api := router.Group("/api")
	{
		api.GET("/rooms", roomHandler.List)
		api.GET("/rooms/:id/messages", roomHandler.GetMessages)

		// Protected routes — require a valid JWT.
		protected := api.Group("/")
		protected.Use(auth.Middleware(cfg.JWTSecret))
		{
			protected.POST("/rooms", roomHandler.Create)
		}
	}

	// WebSocket endpoint — the JWT is passed as a query parameter because
	// browsers do not support custom headers in the WebSocket handshake.
	wsHandler := chat.NewWSHandler(hub)
	router.GET("/ws/:roomID", auth.Middleware(cfg.JWTSecret), wsHandler.ServeWS)

	// ── 5. Start server with graceful shutdown ────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start the server in a goroutine so the main goroutine can listen for
	// OS signals.
	go func() {
		log.Printf("server: listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Block until SIGINT or SIGTERM is received.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("server: shutting down gracefully…")

	// Give in-flight requests up to 5 seconds to complete before forcing close.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server: forced shutdown: %v", err)
	}
	log.Println("server: bye!")
}
