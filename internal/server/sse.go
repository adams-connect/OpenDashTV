package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Client represents a connected browser session listening to the SSE stream.
type Client struct {
	send chan []byte
}

// Hub maintains active SSE client connections and broadcasts widget updates.
// Use sync.RWMutex and non-blocking channel drop rather than unbounded channel bus to prevent goroutine leaks when kiosk browser stalls.
type Hub struct {
	clients map[*Client]struct{}
	mu      sync.RWMutex
}

// NewHub initializes an event broadcasting hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]struct{}),
	}
}

// Register adds a new browser connection to the hub.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

// Unregister removes a disconnected client and cleans up its channel.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// Broadcast dispatches a message to all active clients.
// Stalled clients with full buffers are dropped to prevent memory leaks on the 1GB Pi 3B+.
func (h *Hub) Broadcast(event string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(data))
	payload := []byte(msg)

	for client := range h.clients {
		select {
		case client.send <- payload:
		default:
			// Non-blocking drop: kiosk browser has stalled; avoid accumulating goroutines
			log.Println("[SSE Hub] Buffer full; dropping message for slow/stalled client")
		}
	}
}

// ClientCount returns the number of active SSE listeners.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleEvents streams Server-Sent Events with keep-alive pings.
func (h *Hub) HandleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	client := &Client{
		// Bounded buffer: allows temporary rendering pauses without unbounded RAM growth
		send: make(chan []byte, 16),
	}
	h.Register(client)
	defer h.Unregister(client)

	// Keep-alive ping ticker prevents intermediate NAT routers or Wi-Fi APs from dropping idle links
	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	// Initial handshake
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-client.send:
			if !ok {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		case <-pingTicker.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
