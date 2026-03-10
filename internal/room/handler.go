// Package room provides HTTP handlers for room management.
package room

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samdevgo/go-chat-server/internal/auth"
	"github.com/samdevgo/go-chat-server/internal/models"
	"gorm.io/gorm"
)

// Handler groups the dependencies for room HTTP handlers.
type Handler struct {
	db *gorm.DB
}

// NewHandler constructs a room Handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// createRoomRequest is the expected JSON body for POST /api/rooms.
type createRoomRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

// Create handles POST /api/rooms (requires auth).
// It creates a new named room and returns it as JSON.
func (h *Handler) Create(c *gin.Context) {
	var req createRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetUint(auth.ContextKeyUserID)
	room := models.Room{Name: req.Name, CreatedBy: userID}

	if result := h.db.Create(&room); result.Error != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "room name already taken"})
		return
	}

	c.JSON(http.StatusCreated, room)
}

// List handles GET /api/rooms.
// Returns all rooms ordered by creation date (newest first).
func (h *Handler) List(c *gin.Context) {
	var rooms []models.Room
	h.db.Order("created_at DESC").Find(&rooms)
	c.JSON(http.StatusOK, rooms)
}

// GetMessages handles GET /api/rooms/:id/messages.
// Returns the last 50 messages for the given room, ordered chronologically.
// Pagination can be added via ?before=<message_id> if needed.
func (h *Handler) GetMessages(c *gin.Context) {
	roomID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	var messages []models.Message
	// Fetch messages in a sub-query sorted descending to get the latest 50,
	// then re-sort ascending for display.
	h.db.Where("room_id = ?", roomID).
		Order("created_at DESC").
		Limit(50).
		Find(&messages)

	// Reverse slice for chronological order in the response.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	c.JSON(http.StatusOK, messages)
}
