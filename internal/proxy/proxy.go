package proxy

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

var (
	ErrProxyAlreadyRunning = errors.New("proxy is already running")
	ErrProxyStopped        = errors.New("proxy is stopped")
	ErrToxicNotFound       = errors.New("toxic not found")
	ErrToxicExists         = errors.New("toxic already exists with this name")
)

// Proxy represents a single TCP listener and forwarder with attached fault injection pipelines.
type Proxy struct {
	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`

	Stats *telemetry.Stats `json:"-"`

	mu          sync.RWMutex
	listener    net.Listener
	activeConns map[net.Conn]struct{}
	toxics      map[string]toxics.Toxic
	stopChan    chan struct{}
	running     bool
}

// NewProxy creates an unstarted Proxy definition.
func NewProxy(name, listen, upstream string) *Proxy {
	return &Proxy{
		Name:        name,
		Listen:      listen,
		Upstream:    upstream,
		Enabled:     true,
		Stats:       telemetry.NewStats(),
		activeConns: make(map[net.Conn]struct{}),
		toxics:      make(map[string]toxics.Toxic),
	}
}

// Start binds the listener and initiates the accept loop if enabled.
func (p *Proxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return ErrProxyAlreadyRunning
	}

	if !p.Enabled {
		return nil
	}

	ln, err := net.Listen("tcp", p.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.Listen, err)
	}

	p.listener = ln
	p.stopChan = make(chan struct{})
	p.running = true

	// Update listen address in case port 0 was passed for dynamic allocation
	p.Listen = ln.Addr().String()

	go p.acceptLoop(ln, p.stopChan)
	return nil
}

// Stop closes the listener and terminates all active proxied sockets.
func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false
	close(p.stopChan)

	var firstErr error
	if p.listener != nil {
		firstErr = p.listener.Close()
		p.listener = nil
	}

	// Close all in-flight client connections
	for conn := range p.activeConns {
		_ = conn.Close()
		delete(p.activeConns, conn)
	}

	return firstErr
}

// SetEnabled toggles proxy forwarding. If disabled, current connections are terminated.
func (p *Proxy) SetEnabled(enabled bool) error {
	p.mu.Lock()
	wasRunning := p.running
	p.Enabled = enabled
	p.mu.Unlock()

	if wasRunning && !enabled {
		return p.Stop()
	} else if !wasRunning && enabled {
		return p.Start()
	}
	return nil
}

// acceptLoop receives incoming client connections and dispatches forwarding goroutines.
func (p *Proxy) acceptLoop(ln net.Listener, stopChan <-chan struct{}) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-stopChan:
				return
			default:
				p.Stats.RecordError()
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		p.mu.Lock()
		if !p.running {
			p.mu.Unlock()
			_ = conn.Close()
			return
		}
		p.activeConns[conn] = struct{}{}
		upToxics := p.getToxicsForStreamLocked(toxics.Upstream)
		downToxics := p.getToxicsForStreamLocked(toxics.Downstream)
		upstreamAddr := p.Upstream
		p.mu.Unlock()

		go func(c net.Conn) {
			defer func() {
				p.mu.Lock()
				delete(p.activeConns, c)
				p.mu.Unlock()
			}()

			_ = PipeConnection(c, upstreamAddr, p.Stats, upToxics, downToxics, 5*time.Second)
		}(conn)
	}
}

// AddToxic registers a new toxic or returns an error if already present.
func (p *Proxy) AddToxic(t toxics.Toxic) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.toxics[t.Name()]; exists {
		return ErrToxicExists
	}
	p.toxics[t.Name()] = t
	return nil
}

// UpdateToxic updates an existing toxic configuration.
func (p *Proxy) UpdateToxic(name string, attrs map[string]interface{}, toxicity float64) (toxics.Toxic, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	t, exists := p.toxics[name]
	if !exists {
		return nil, ErrToxicNotFound
	}

	if attrs != nil {
		if err := t.SetAttributes(attrs); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// RemoveToxic deletes a toxic by name.
func (p *Proxy) RemoveToxic(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.toxics[name]; !exists {
		return ErrToxicNotFound
	}
	delete(p.toxics, name)
	return nil
}

// GetToxic retrieves a toxic by name.
func (p *Proxy) GetToxic(name string) (toxics.Toxic, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	t, ok := p.toxics[name]
	return t, ok
}

// GetToxics returns a slice of all active toxics.
func (p *Proxy) GetToxics() []toxics.Toxic {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]toxics.Toxic, 0, len(p.toxics))
	for _, t := range p.toxics {
		result = append(result, t)
	}
	return result
}

// ResetToxics clears all configured toxics.
func (p *Proxy) ResetToxics() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.toxics = make(map[string]toxics.Toxic)
}

func (p *Proxy) getToxicsForStreamLocked(dir toxics.Direction) []toxics.Toxic {
	var list []toxics.Toxic
	for _, t := range p.toxics {
		if t.Stream() == dir {
			list = append(list, t)
		}
	}
	return list
}

// IsRunning reports whether the proxy is currently listening.
func (p *Proxy) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}
