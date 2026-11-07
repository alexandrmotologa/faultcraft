package telemetry

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Stats tracks real-time traffic telemetry and latency distributions for a proxy.
type Stats struct {
	activeConns     int64
	totalConns      uint64
	bytesUpstream   uint64
	bytesDownstream uint64
	errorCount      uint64
	faultCount      uint64

	mu          sync.RWMutex
	latencies   []time.Duration
	maxSamples  int
	lastUpdated time.Time
}

// NewStats initializes a Stats tracker with a fixed sample reservoir.
func NewStats() *Stats {
	return &Stats{
		latencies:   make([]time.Duration, 0, 1024),
		maxSamples:  1024,
		lastUpdated: time.Now(),
	}
}

// ConnOpened increments the active and total connection counters.
func (s *Stats) ConnOpened() {
	atomic.AddInt64(&s.activeConns, 1)
	atomic.AddUint64(&s.totalConns, 1)
}

// ConnClosed decrements the active connection counter.
func (s *Stats) ConnClosed() {
	atomic.AddInt64(&s.activeConns, -1)
}

// AddBytesUpstream records transferred bytes from client to upstream.
func (s *Stats) AddBytesUpstream(n int64) {
	if n > 0 {
		atomic.AddUint64(&s.bytesUpstream, uint64(n))
	}
}

// AddBytesDownstream records transferred bytes from upstream to client.
func (s *Stats) AddBytesDownstream(n int64) {
	if n > 0 {
		atomic.AddUint64(&s.bytesDownstream, uint64(n))
	}
}

// RecordError increments the connection error counter.
func (s *Stats) RecordError() {
	atomic.AddUint64(&s.errorCount, 1)
}

// RecordFault increments the injected fault counter.
func (s *Stats) RecordFault() {
	atomic.AddUint64(&s.faultCount, 1)
}

// RecordLatency records a latency observation for percentile estimation.
func (s *Stats) RecordLatency(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.latencies) >= s.maxSamples {
		s.latencies = s.latencies[1:]
	}
	s.latencies = append(s.latencies, d)
	s.lastUpdated = time.Now()
}

// Snapshot represents a read-only telemetry snapshot.
type Snapshot struct {
	ActiveConnections int64   `json:"active_connections"`
	TotalConnections  uint64  `json:"total_connections"`
	BytesUpstream     uint64  `json:"bytes_upstream"`
	BytesDownstream   uint64  `json:"bytes_downstream"`
	ErrorCount        uint64  `json:"error_count"`
	FaultCount        uint64  `json:"fault_count"`
	P50LatencyMs      float64 `json:"p50_latency_ms"`
	P90LatencyMs      float64 `json:"p90_latency_ms"`
	P99LatencyMs      float64 `json:"p99_latency_ms"`
}

// Snapshot returns the current metrics values and calculates latency percentiles.
func (s *Stats) Snapshot() Snapshot {
	s.mu.RLock()
	n := len(s.latencies)
	var p50, p90, p99 float64
	if n > 0 {
		sorted := make([]time.Duration, n)
		copy(sorted, s.latencies)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})

		p50 = float64(sorted[int(math.Floor(float64(n)*0.50))].Milliseconds())
		p90 = float64(sorted[int(math.Floor(float64(n)*0.90))].Milliseconds())
		p99Idx := int(math.Floor(float64(n) * 0.99))
		if p99Idx >= n {
			p99Idx = n - 1
		}
		p99 = float64(sorted[p99Idx].Milliseconds())
	}
	s.mu.RUnlock()

	return Snapshot{
		ActiveConnections: atomic.LoadInt64(&s.activeConns),
		TotalConnections:  atomic.LoadUint64(&s.totalConns),
		BytesUpstream:     atomic.LoadUint64(&s.bytesUpstream),
		BytesDownstream:   atomic.LoadUint64(&s.bytesDownstream),
		ErrorCount:        atomic.LoadUint64(&s.errorCount),
		FaultCount:        atomic.LoadUint64(&s.faultCount),
		P50LatencyMs:      p50,
		P90LatencyMs:      p90,
		P99LatencyMs:      p99,
	}
}

// Reset clears all counters and latency observations.
func (s *Stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	atomic.StoreInt64(&s.activeConns, 0)
	atomic.StoreUint64(&s.totalConns, 0)
	atomic.StoreUint64(&s.bytesUpstream, 0)
	atomic.StoreUint64(&s.bytesDownstream, 0)
	atomic.StoreUint64(&s.errorCount, 0)
	atomic.StoreUint64(&s.faultCount, 0)
	s.latencies = s.latencies[:0]
	s.lastUpdated = time.Now()
}
