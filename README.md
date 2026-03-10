<div align="center">

# 💬 GoChat

**A production-ready real-time chat server written in Go**

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go)
![Gin](https://img.shields.io/badge/Gin-v1.10-blue?style=flat-square)
![WebSocket](https://img.shields.io/badge/WebSocket-gorilla-orange?style=flat-square)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat-square&logo=docker)

</div>

---

## ✨ Features

- **Real-time messaging** via WebSocket — messages appear instantly across all connected clients
- **JWT authentication** — secure register/login with bcrypt password hashing
- **Persistent message history** — PostgreSQL stores all messages; history loads on room join
- **Multiple chat rooms** — create and switch between named rooms
- **Concurrent hub architecture** — idiomatic Go concurrency (goroutines + channels, no locks in the hot path)
- **Automatic reconnect** — exponential backoff reconnection on WebSocket disconnect
- **Multi-stage Docker build** — ~20 MB final image, ready for any container platform
- **Dark-themed frontend** — built with vanilla HTML/CSS/JS, no framework needed

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      HTTP / WS Client                   │
└──────────────────────────┬──────────────────────────────┘
                           │
                    ┌──────▼──────┐
                    │  Gin Router  │  REST API + WS upgrade
                    └──────┬──────┘
          ┌────────────────┼─────────────────┐
          │                │                 │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
   │ Auth Handler│  │ Room Handler│  │  WS Handler  │
   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
          │                │                 │
          │                │         ┌───────▼────────┐
          │                │         │      Hub        │ ← single goroutine
          │                │         │  (channels)     │
          │                │         └───────┬────────┘
          │                │                 │ fan-out
          └────────────────┴─────────────────▼
                           │
                    ┌──────▼──────┐
                    │ PostgreSQL  │  users · rooms · messages
                    └─────────────┘
```

### Concurrency Model

The `Hub` runs in a **single goroutine** and owns the `rooms` map exclusively. All state changes (register / unregister / broadcast) are sent through buffered channels. This eliminates data races on the connection map without needing explicit locks, which is idiomatic Go.

Each WebSocket `Client` spawns two goroutines:
- **readPump** — reads frames from the WS connection → sends to Hub channel
- **writePump** — receives from a per-client `send` channel → writes to WS connection

---

## 🚀 Quick Start

### Prerequisites
- [Docker](https://www.docker.com/) & Docker Compose

### 1. Clone the repository

```bash
git clone https://github.com/YOUR_USERNAME/go-chat-server.git
cd go-chat-server
```

### 2. Start with Docker Compose

```bash
docker-compose up --build
```

The app will be available at **http://localhost:8080**

### 3. (Optional) Run locally without Docker

```bash
# Install Go 1.22+, then:
cp .env.example .env
# Edit .env with your local PostgreSQL credentials

go mod download
go run ./cmd/server
```

---

## 📡 API Reference

### Auth

| Method | Endpoint | Body | Auth |
|--------|----------|------|------|
| `POST` | `/api/auth/register` | `{ username, password }` | — |
| `POST` | `/api/auth/login` | `{ username, password }` | — |

Both return: `{ token, user_id, username }`

### Rooms

| Method | Endpoint | Auth |
|--------|----------|------|
| `GET` | `/api/rooms` | — |
| `POST` | `/api/rooms` | Bearer JWT |
| `GET` | `/api/rooms/:id/messages` | — |

### WebSocket

```
GET /ws/:roomID?token=<JWT>
```

After connecting, send plain text frames. Receive JSON frames:
```json
{
  "id": 42,
  "room_id": 1,
  "user_id": 7,
  "username": "alice",
  "content": "Hello!",
  "created_at": "2024-03-10T18:00:00Z"
}
```

---

## 📁 Project Structure

```
go-chat-server/
├── cmd/server/main.go          # Entry point: router, hub, graceful shutdown
├── internal/
│   ├── config/config.go        # Typed env-var configuration
│   ├── db/postgres.go          # GORM connection + auto-migration
│   ├── models/models.go        # User, Room, Message structs
│   ├── auth/
│   │   ├── service.go          # JWT generation & validation
│   │   ├── handler.go          # Register & Login HTTP handlers
│   │   └── middleware.go       # Gin JWT auth middleware
│   ├── room/handler.go         # Room CRUD + message history
│   └── chat/
│       ├── hub.go              # Central message broker (channels)
│       ├── client.go           # WebSocket client (readPump / writePump)
│       └── handler.go          # WS upgrade endpoint
├── web/
│   ├── index.html              # Login / Register page
│   ├── chat.html               # Chat UI
│   ├── style.css               # Dark theme
│   └── app.js                  # WS client, API calls, reconnect logic
├── Dockerfile                  # Multi-stage build
├── docker-compose.yml          # App + PostgreSQL
└── .env.example                # Environment variable template
```

---

## 🔒 Security Notes

- Passwords hashed with **bcrypt** (cost 12)
- JWT signed with **HMAC-SHA256**; signing algorithm explicitly validated to prevent `alg:none` attacks
- Generic error messages on login failure (no username enumeration)
- `CheckOrigin` in the WebSocket upgrader should be restricted to your domain in production

---

## 📄 License

MIT © 2024
