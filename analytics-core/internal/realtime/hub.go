// Package realtime transmite eventos ao vivo para o painel administrativo.
//
// O blueprint original especifica WebSocket. Implementamos aqui via
// Server-Sent Events (SSE), que cumpre o mesmo papel (push unidirecional
// Core -> painel) usando apenas net/http da standard library — sem
// dependência de gorilla/websocket ou nhooyr.io/websocket. Se no futuro
// você precisar de comunicação bidirecional real, troque este hub por
// WebSocket usando uma dessas libs (via `go get`, fora deste ambiente).
package realtime

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Event é uma unidade de dado transmitida ao painel (ex: contadores
// agregados de 1s: requests/s, taxa de erro, latência p95).
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan Event]struct{})}
}

// Broadcast envia um evento para todos os painéis conectados no momento.
// Non-blocking: um cliente lento não atrasa os demais nem o emissor.
func (h *Hub) Broadcast(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			// cliente lento demais, descarta este evento para ele
		}
	}
}

// ServeHTTP expõe o endpoint SSE (ex: GET /api/realtime/events).
// Requer autenticação de admin antes de chegar aqui (ver gateway/handler.go
// ou um middleware dedicado no servidor do painel).
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming não suportado", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan Event, 32)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			w.Write([]byte("data: "))
			w.Write(b)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}
