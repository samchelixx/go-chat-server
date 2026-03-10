// Package chat contains the WebSocket upgrade handler.
package chat

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samdevgo/go-chat-server/internal/auth"
)

// upgrader configures the WebSocket upgrade with a permissive origin check.
// In production you would restrict CheckOrigin to your actual domain.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development. Replace this with a strict
		// allowlist check (e.g., r.Header.Get("Origin") == "https://yourapp.com")
		// before deploying to production.
		return true
	},
}

// Handler handles the WebSocket upgrade endpoint.
type WSHandler struct {
	hub *Hub
}

// NewWSHandler constructs a WSHandler bound to the given Hub.
func NewWSHandler(hub *Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// ServeWS handles GET /ws/:roomID.
// It upgrades the connection to WebSocket, then creates and registers a Client.
func (h *WSHandler) ServeWS(c *gin.Context) {
	// Parse the room ID from the URL parameter.
	roomID, err := strconv.ParseUint(c.Param("roomID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	// Retrieve the authenticated user from context (set by JWT middleware).
	userID := c.GetUint(auth.ContextKeyUserID)
	username := c.GetString(auth.ContextKeyUsername)

	// Upgrade the HTTP connection to a WebSocket connection.
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade already wrote an error response if it fails, so just log.
		return
	}

	// Create the client and register it with the Hub.
	client := NewClient(h.hub, conn, uint(roomID), userID, username)
	h.hub.register <- client
}
