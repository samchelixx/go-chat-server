// Package models defines the database models (GORM entities) for the chat server.
// Each struct in this package maps directly to a PostgreSQL table.
package models

import "time"

// User represents a registered chat participant.
// Passwords are never stored in plain text — only the bcrypt hash is persisted.
type User struct {
	ID           uint      `gorm:"primaryKey"                  json:"id"`
	Username     string    `gorm:"uniqueIndex;not null;size:50" json:"username"`
	PasswordHash string    `gorm:"not null"                    json:"-"` // never expose hash in JSON
	CreatedAt    time.Time `                                   json:"created_at"`
}

// Room is a named chat channel. Users can create rooms and join them via WebSocket.
type Room struct {
	ID        uint      `gorm:"primaryKey"                  json:"id"`
	Name      string    `gorm:"uniqueIndex;not null;size:100" json:"name"`
	CreatedBy uint      `gorm:"not null"                    json:"created_by"`
	CreatedAt time.Time `                                   json:"created_at"`
	// Messages is not pre-loaded by default; use explicit Preload when needed.
	Messages []Message `gorm:"foreignKey:RoomID"           json:"-"`
}

// Message stores a single chat message, linked to the room and the sender.
// It is persisted on every WebSocket broadcast so clients can view history.
type Message struct {
	ID        uint      `gorm:"primaryKey"      json:"id"`
	RoomID    uint      `gorm:"index;not null"  json:"room_id"`
	UserID    uint      `gorm:"not null"        json:"user_id"`
	Username  string    `gorm:"not null;size:50" json:"username"` // denormalised for fast display
	Content   string    `gorm:"not null"        json:"content"`
	CreatedAt time.Time `                       json:"created_at"`
}
