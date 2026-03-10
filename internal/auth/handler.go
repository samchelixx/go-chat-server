// Package auth contains the HTTP handlers for user registration and login.
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samdevgo/go-chat-server/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Handler groups the dependencies needed by auth HTTP handlers.
type Handler struct {
	db           *gorm.DB
	jwtSecret    string
	jwtExpiresH  int
}

// NewHandler constructs an auth Handler with the given dependencies.
func NewHandler(db *gorm.DB, secret string, expiresHours int) *Handler {
	return &Handler{db: db, jwtSecret: secret, jwtExpiresH: expiresHours}
}

// registerRequest is the expected JSON body for POST /api/auth/register.
type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

// loginRequest is the expected JSON body for POST /api/auth/login.
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// tokenResponse is returned on successful auth operations.
type tokenResponse struct {
	Token    string `json:"token"`
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
}

// Register handles POST /api/auth/register.
// It validates input, hashes the password, stores the user, and returns a JWT.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash the password with bcrypt cost 12 — secure but still fast enough for login.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
	}

	// GORM will return a unique constraint violation if the username is already taken.
	if result := h.db.Create(&user); result.Error != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	}

	// Issue a JWT immediately so the client is logged in right after registration.
	token, err := GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiresH)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, tokenResponse{Token: token, UserID: user.ID, Username: user.Username})
}

// Login handles POST /api/auth/login.
// It verifies credentials and returns a fresh JWT on success.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if result := h.db.Where("username = ?", req.Username).First(&user); result.Error != nil {
		// Return a generic error to avoid leaking whether the username exists.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// CompareHashAndPassword is constant-time, preventing timing attacks.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, err := GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiresH)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, tokenResponse{Token: token, UserID: user.ID, Username: user.Username})
}
