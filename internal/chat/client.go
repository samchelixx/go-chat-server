// Package chat contains the Client — a WebSocket connection wrapper.
//
// Each connected user has one Client per room.
// Two goroutines run per client:
//   - readPump:  reads frames from the WebSocket and sends them to the Hub.
//   - writePump: reads from the send channel and writes frames to the WebSocket.
//
// Separating reads and writes into their own goroutines is idiomatic in Go
// and the standard pattern recommended by the gorilla/websocket library.
package chat

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/samdevgo/go-chat-server/internal/models"
)

const (
	// writeWait is the allowed time to write a message to the WebSocket peer.
	writeWait = 10 * time.Second

	// pongWait is how long the server will wait for a pong response to a ping.
	pongWait = 60 * time.Second

	// pingPeriod controls how often the server sends a ping to the client.
	// Must be less than pongWait to ensure the peer is detected as dead in time.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum message size in bytes the server will accept.
	maxMessageSize = 4096
)

// Client wraps a WebSocket connection for a single user in a single room.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan models.Message // buffered channel of outbound messages
	RoomID   uint
	UserID   uint
	Username string
}

// NewClient creates a Client and immediately starts its read and write pumps.
func NewClient(hub *Hub, conn *websocket.Conn, roomID, userID uint, username string) *Client {
	c := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan models.Message, 256),
		RoomID:   roomID,
		UserID:   userID,
		Username: username,
	}
	go c.writePump()
	go c.readPump()
	return c
}

// readPump pumps incoming WebSocket messages to the Hub's broadcast channel.
// It runs until the connection closes, then unregisters the client.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	// If no message (including pong) is received within pongWait, the connection
	// is considered dead and the read will return an error.
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		// Reset the read deadline each time we receive a pong.
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		// We only care about the message text; the message type is always TextMessage.
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("client: unexpected close for user %q: %v", c.Username, err)
			}
			break // exit the loop → deferred cleanup runs
		}

		// Build a Message struct; the Hub will persist it and broadcast it.
		msg := models.Message{
			RoomID:   c.RoomID,
			UserID:   c.UserID,
			Username: c.Username,
			Content:  string(data),
		}
		c.hub.broadcast <- &OutMessage{RoomID: c.RoomID, Message: msg}
	}
}

// writePump pumps messages from the send channel to the WebSocket connection.
// A ping ticker keeps the connection alive across idle periods.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The Hub closed the channel — send a close frame.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			// Write the message as JSON for easy parsing on the frontend.
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("client: write error for user %q: %v", c.Username, err)
				return
			}

		case <-ticker.C:
			// Send a ping to detect dead connections without waiting for a read error.
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
