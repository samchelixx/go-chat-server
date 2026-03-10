// Package chat contains the Hub — the central message broker for the chat server.
//
// Architecture:
//
//	Each room has its own map of connected clients.
//	The Hub runs in a single dedicated goroutine and owns all mutable state,
//	which means no locks are needed to manage the client maps — coordination
//	happens purely through channels. This is idiomatic Go concurrency.
package chat

import (
	"log"
	"sync"

	"github.com/samdevgo/go-chat-server/internal/models"
	"gorm.io/gorm"
)

// Hub maintains the set of active WebSocket clients grouped by room.
// All mutations to the rooms map go through the register/unregister channels,
// so the Run() goroutine is the sole owner of that state.
type Hub struct {
	// rooms maps roomID → set of clients connected to that room.
	rooms map[uint]map[*Client]bool

	// broadcast receives outgoing messages that need to be fanned out to a room.
	broadcast chan *OutMessage

	// register / unregister handle client lifecycle events.
	register   chan *Client
	unregister chan *Client

	// mu protects rooms against rare concurrent reads from outside Run().
	mu sync.RWMutex

	// db is used to persist messages to PostgreSQL for history retrieval.
	db *gorm.DB
}

// OutMessage wraps a chat message with its target room, so the Hub knows
// which set of clients to broadcast to.
type OutMessage struct {
	RoomID  uint
	Message models.Message
}

// NewHub constructs and returns a Hub ready to be started with Run().
func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		rooms:      make(map[uint]map[*Client]bool),
		broadcast:  make(chan *OutMessage, 256), // buffered to avoid blocking senders
		register:   make(chan *Client),
		unregister: make(chan *Client),
		db:         db,
	}
}

// Run starts the Hub's event loop. It must be called in its own goroutine.
// It is the only goroutine that reads and writes the rooms map, eliminating
// the need for locks in the hot path.
func (h *Hub) Run() {
	for {
		select {

		case client := <-h.register:
			// Add the client to its room's client set. Create the room bucket
			// on first connection.
			h.mu.Lock()
			if _, ok := h.rooms[client.RoomID]; !ok {
				h.rooms[client.RoomID] = make(map[*Client]bool)
			}
			h.rooms[client.RoomID][client] = true
			h.mu.Unlock()
			log.Printf("hub: client %q joined room %d (total: %d)",
				client.Username, client.RoomID, len(h.rooms[client.RoomID]))

		case client := <-h.unregister:
			// Remove the client and close its send channel. If the room is now
			// empty, remove the bucket to free memory.
			h.mu.Lock()
			if clients, ok := h.rooms[client.RoomID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.rooms, client.RoomID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("hub: client %q left room %d", client.Username, client.RoomID)

		case out := <-h.broadcast:
			// Persist the message to the DB before sending so that even if a
			// write to a client fails, the message is not lost.
			if err := h.db.Create(&out.Message).Error; err != nil {
				log.Printf("hub: failed to persist message: %v", err)
			}

			// Fan-out: deliver the message to every client in the target room.
			h.mu.RLock()
			clients := h.rooms[out.RoomID]
			h.mu.RUnlock()

			for client := range clients {
				select {
				case client.send <- out.Message:
					// Message queued for delivery.
				default:
					// Client's send buffer is full (likely a slow connection).
					// Close and remove it rather than blocking the broadcast.
					close(client.send)
					h.mu.Lock()
					delete(h.rooms[out.RoomID], client)
					h.mu.Unlock()
				}
			}
		}
	}
}
