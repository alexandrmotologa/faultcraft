package toxics

import (
	"sync"
	"time"
)

func init() {
	Register("reset_peer", NewResetToxic)
}

// ResetToxic terminates the socket using a TCP RST packet.
type ResetToxic struct {
	BaseToxic

	mu      sync.Mutex
	Timeout int64 `json:"timeout"` // Milliseconds to wait before issuing RST
}

// NewResetToxic constructs a ResetToxic.
func NewResetToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &ResetToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "reset_peer",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		Timeout: 0,
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *ResetToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if timeout, ok := attrs["timeout"]; ok {
		t.Timeout = toInt64(timeout)
	}
	return nil
}

func (t *ResetToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &ResetToxic{
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

func (t *ResetToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
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
	waitMs := t.Timeout
	t.mu.Unlock()

	if waitMs > 0 {
		select {
		case <-link.Done():
			return
		case <-time.After(time.Duration(waitMs) * time.Millisecond):
		}
	}

	link.Stats.RecordFault()
	link.Reset()
}
