package toxics

import (
	"fmt"
	"sync"
	"time"
)

func init() {
	Register("circuit_breaker_probe", NewCircuitBreakerToxic)
}

type CBState string

const (
	StateClosed   CBState = "closed"    // Tripping phase: injects errors to force client to trip
	StateOpen     CBState = "open"      // Cooldown phase: rejects fast while client breaker should be open
	StateHalfOpen CBState = "half_open" // Probe phase: allows trial requests to verify client recovery
)

// CircuitBreakerToxic simulates an upstream service failure sequence to test client circuit breakers.
type CircuitBreakerToxic struct {
	BaseToxic

	mu              sync.Mutex
	TripAfterErrors int64   `json:"trip_after_errors"` // Consecutive errors injected before opening
	CooldownMs      int64   `json:"cooldown_ms"`       // Duration to remain in Open state before Half-Open
	ProbeSuccesses  int64   `json:"probe_successes"`   // Number of clean requests allowed in Half-Open to recover
	StatusCode      int     `json:"status_code"`       // HTTP status code to return during failure (default 503)

	state        CBState
	errorCount   int64
	openedAt     time.Time
	successCount int64
}

// NewCircuitBreakerToxic constructs a CircuitBreakerToxic.
func NewCircuitBreakerToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &CircuitBreakerToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "circuit_breaker_probe",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		TripAfterErrors: 5,
		CooldownMs:      3000,
		ProbeSuccesses:  2,
		StatusCode:      503,
		state:           StateClosed,
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *CircuitBreakerToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if trip, ok := attrs["trip_after_errors"]; ok {
		t.TripAfterErrors = toInt64(trip)
	}
	if cd, ok := attrs["cooldown_ms"]; ok {
		t.CooldownMs = toInt64(cd)
	}
	if probe, ok := attrs["probe_successes"]; ok {
		t.ProbeSuccesses = toInt64(probe)
	}
	if status, ok := attrs["status_code"]; ok {
		t.StatusCode = int(toInt64(status))
	}
	if t.TripAfterErrors <= 0 {
		t.TripAfterErrors = 5
	}
	if t.CooldownMs <= 0 {
		t.CooldownMs = 3000
	}
	if t.StatusCode < 400 || t.StatusCode > 599 {
		t.StatusCode = 503
	}
	return nil
}

func (t *CircuitBreakerToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &CircuitBreakerToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		TripAfterErrors: t.TripAfterErrors,
		CooldownMs:      t.CooldownMs,
		ProbeSuccesses:  t.ProbeSuccesses,
		StatusCode:      t.StatusCode,
		state:           t.state,
		errorCount:      t.errorCount,
		openedAt:        t.openedAt,
		successCount:    t.successCount,
	}
}

func (t *CircuitBreakerToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		t.mu.Lock()
		currentState := t.state
		cooldown := time.Duration(t.CooldownMs) * time.Millisecond

		// Evaluate state transitions
		if currentState == StateOpen && time.Since(t.openedAt) >= cooldown {
			t.state = StateHalfOpen
			t.successCount = 0
			currentState = StateHalfOpen
		}

		shouldFail := false
		if currentState == StateClosed {
			if t.errorCount < t.TripAfterErrors {
				shouldFail = true
				t.errorCount++
				if t.errorCount >= t.TripAfterErrors {
					t.state = StateOpen
					t.openedAt = time.Now()
				}
			}
		} else if currentState == StateOpen {
			shouldFail = true
		} else if currentState == StateHalfOpen {
			// Allow probe request
			t.successCount++
			if t.successCount >= t.ProbeSuccesses {
				// Re-closed / recovered
				t.state = StateClosed
				t.errorCount = 0
			}
		}

		code := t.StatusCode
		t.mu.Unlock()

		if shouldFail && t.ShouldApply() {
			link.Stats.RecordFault()

			if isHTTPResponse(chunk.Data) {
				body := fmt.Sprintf(`{"error":"circuit_breaker_tripped","state":"%s","status":%d}`+"\n", currentState, code)
				respStr := fmt.Sprintf(
					"HTTP/1.1 %d Service Unavailable\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
					code,
					len(body),
					body,
				)
				select {
				case <-link.Done():
					return
				case out <- &Chunk{Data: []byte(respStr), Timestamp: chunk.Timestamp}:
				}
				continue
			} else {
				// Raw TCP: abort via RST
				link.Reset()
				return
			}
		}

		select {
		case <-link.Done():
			return
		case out <- chunk:
		}
	}
}
