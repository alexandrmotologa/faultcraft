package toxics

import (
	"sync"
	"time"
)

func init() {
	Register("timeout", NewTimeoutToxic)
}

// TimeoutToxic closes the connection after a specified duration in milliseconds.
type TimeoutToxic struct {
	BaseToxic

	mu      sync.Mutex
	Timeout int64 `json:"timeout"` // Timeout in milliseconds
}

// NewTimeoutToxic constructs a TimeoutToxic.
func NewTimeoutToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &TimeoutToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "timeout",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		Timeout: 1000, // Default 1000ms
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TimeoutToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if timeout, ok := attrs["timeout"]; ok {
		t.Timeout = toInt64(timeout)
	}
	return nil
}

func (t *TimeoutToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &TimeoutToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		Timeout: t.Timeout,
	}
}

func (t *TimeoutToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	if !t.ShouldApply() {
		for chunk := range in {
			if link.IsClosed() {
				return
			}
			out <- chunk
		}
		return
	}

	t.mu.Lock()
	timeoutDur := time.Duration(t.Timeout) * time.Millisecond
	t.mu.Unlock()

	timer := time.NewTimer(timeoutDur)
	defer timer.Stop()

	for {
		select {
		case <-link.Done():
			return
		case <-timer.C:
			link.Stats.RecordFault()
			link.Close()
			return
		case chunk, ok := <-in:
			if !ok {
				return
			}
			select {
			case <-link.Done():
				return
			case out <- chunk:
			}
		}
	}
}
