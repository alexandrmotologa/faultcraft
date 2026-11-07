package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

// SSEBroker manages real-time Server-Sent Events subscribers.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
	manager *proxy.Manager
	stop    chan struct{}
}

// NewSSEBroker initializes an SSE broadcaster and starts periodic telemetry updates.
func NewSSEBroker(mgr *proxy.Manager) *SSEBroker {
	b := &SSEBroker{
		clients: make(map[chan []byte]struct{}),
		manager: mgr,
		stop:    make(chan struct{}),
	}
	go b.telemetryLoop()
	return b
}

// Close terminates the SSE broadcast loop.
func (b *SSEBroker) Close() {
	close(b.stop)
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		close(ch)
	}
	b.clients = make(map[chan []byte]struct{})
}

func (b *SSEBroker) subscribe() chan []byte {
	ch := make(chan []byte, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *SSEBroker) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[ch]; ok {
		delete(b.clients, ch)
		close(ch)
	}
}

// Broadcast sends payload to all active SSE subscribers.
func (b *SSEBroker) Broadcast(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- data:
		default:
		}
	}
}

// ServeHTTP streams Server-Sent Events to web clients.
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	// Send initial snapshot immediately
	initialData := b.gatherTelemetry()
	fmt.Fprintf(w, "data: %s\n\n", initialData)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

type TelemetryPayload struct {
	Timestamp int64            `json:"timestamp"`
	Proxies   []ProxyTelemetry `json:"proxies"`
}

type ProxyTelemetry struct {
	Name      string             `json:"name"`
	Listen    string             `json:"listen"`
	Upstream  string             `json:"upstream"`
	Enabled   bool               `json:"enabled"`
	Stats     telemetry.Snapshot `json:"stats"`
	ToxicList []toxics.BaseToxic `json:"toxics"`
}

func (b *SSEBroker) gatherTelemetry() []byte {
	proxies := b.manager.List()
	payload := TelemetryPayload{
		Timestamp: time.Now().Unix(),
		Proxies:   make([]ProxyTelemetry, 0, len(proxies)),
	}

	for _, p := range proxies {
		toxList := p.GetToxics()
		baseToxics := make([]toxics.BaseToxic, 0, len(toxList))
		for _, t := range toxList {
			baseToxics = append(baseToxics, toxics.BaseToxic{
				ToxicName: t.Name(),
				ToxicType: t.Type(),
				ToxicDir:  t.Stream(),
				Prob:      t.Toxicity(),
				Attrs:     t.Attributes(),
			})
		}

		payload.Proxies = append(payload.Proxies, ProxyTelemetry{
			Name:      p.Name,
			Listen:    p.Listen,
			Upstream:  p.Upstream,
			Enabled:   p.Enabled,
			Stats:     p.Stats.Snapshot(),
			ToxicList: baseToxics,
		})
	}

	raw, _ := json.Marshal(payload)
	return raw
}

func (b *SSEBroker) telemetryLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			data := b.gatherTelemetry()
			b.Broadcast(data)
		}
	}
}
