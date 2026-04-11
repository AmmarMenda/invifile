package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	// upgrader turns an HTTP connection into a WebSocket connection
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true }, // Allows all devices on network
	}

	// State management
	clients      = make(map[*websocket.Conn]bool)
	clientsMutex sync.Mutex // Prevents crashes when multiple people connect at once
	currentText  = ""       // This holds our "Temporary Clipboard"
)

func HandleClipboard(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 1. Register new client
	clientsMutex.Lock()
	clients[conn] = true
	clientsMutex.Unlock()

	// 2. Immediately send the current clipboard content to the new user
	conn.WriteMessage(websocket.TextMessage, []byte(currentText))

	// 3. Listen for updates from this user
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			clientsMutex.Lock()
			delete(clients, conn)
			clientsMutex.Unlock()
			break
		}

		// Update global state
		currentText = string(message)

		// 4. Broadcast the update to EVERYONE else
		broadcast(message)
	}
}

func broadcast(msg []byte) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		client.WriteMessage(websocket.TextMessage, msg)
	}
}
