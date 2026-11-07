package proxy

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrProxyNotFound = errors.New("proxy not found")
	ErrProxyExists   = errors.New("proxy already exists")
)

// Manager coordinates the lifecycle of multiple active proxies.
type Manager struct {
	mu      sync.RWMutex
	proxies map[string]*Proxy
}

// NewManager initializes an empty proxy collection.
func NewManager() *Manager {
	return &Manager{
		proxies: make(map[string]*Proxy),
	}
}

// Add registers and starts a new proxy.
func (m *Manager) Add(p *Proxy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.proxies[p.Name]; exists {
		return ErrProxyExists
	}

	if err := p.Start(); err != nil {
		return err
	}

	m.proxies[p.Name] = p
	return nil
}

// Get finds a proxy by name.
func (m *Manager) Get(name string) (*Proxy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.proxies[name]
	return p, ok
}

// Delete terminates and removes a proxy.
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	p, ok := m.proxies[name]
	if !ok {
		m.mu.Unlock()
		return ErrProxyNotFound
	}
	delete(m.proxies, name)
	m.mu.Unlock()

	return p.Stop()
}

// List returns all registered proxies.
func (m *Manager) List() []*Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Proxy, 0, len(m.proxies))
	for _, p := range m.proxies {
		result = append(result, p)
	}
	return result
}

// Reset clears all toxics from all proxies and ensures all proxies are enabled.
func (m *Manager) Reset() error {
	m.mu.RLock()
	proxies := make([]*Proxy, 0, len(m.proxies))
	for _, p := range m.proxies {
		proxies = append(proxies, p)
	}
	m.mu.RUnlock()

	for _, p := range proxies {
		p.ResetToxics()
		if !p.IsRunning() && p.Enabled {
			if err := p.Start(); err != nil {
				return fmt.Errorf("failed to restart proxy %s: %w", p.Name, err)
			}
		}
	}
	return nil
}

// Clear stops all proxies and purges the registry.
func (m *Manager) Clear() {
	m.mu.Lock()
	proxies := make([]*Proxy, 0, len(m.proxies))
	for _, p := range m.proxies {
		proxies = append(proxies, p)
	}
	m.proxies = make(map[string]*Proxy)
	m.mu.Unlock()

	for _, p := range proxies {
		_ = p.Stop()
	}
}

// Populate adds a batch of proxies, commonly used during test suite bootstrapping.
func (m *Manager) Populate(proxies []*Proxy) error {
	for _, p := range proxies {
		if err := m.Add(p); err != nil {
			return err
		}
	}
	return nil
}
