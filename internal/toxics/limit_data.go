package toxics

import (
	"sync"
)

func init() {
	Register("limit_data", NewLimitDataToxic)
}

// LimitDataToxic closes the connection once transferred bytes reach a specified quota.
type LimitDataToxic struct {
	BaseToxic

	mu    sync.Mutex
	Bytes int64 `json:"bytes"` // Quota in bytes
	seen  int64
}

// NewLimitDataToxic constructs a LimitDataToxic.
func NewLimitDataToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &LimitDataToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "limit_data",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		Bytes: 1024,
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *LimitDataToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if b, ok := attrs["bytes"]; ok {
		t.Bytes = toInt64(b)
	}
	return nil
}

func (t *LimitDataToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &LimitDataToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		Bytes: t.Bytes,
	}
}

func (t *LimitDataToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		if !t.ShouldApply() {
			select {
			case <-link.Done():
				return
			case out <- chunk:
			}
			continue
		}

		t.mu.Lock()
		remaining := t.Bytes - t.seen
		t.mu.Unlock()

		chunkLen := int64(len(chunk.Data))
		if remaining <= 0 {
			link.Stats.RecordFault()
			link.Close()
			return
		}

		if chunkLen > remaining {
			// Deliver only up to quota then terminate
			link.Stats.RecordFault()
			subChunk := &Chunk{
				Data:      chunk.Data[:remaining],
				Timestamp: chunk.Timestamp,
			}
			select {
			case <-link.Done():
				return
			case out <- subChunk:
			}
			link.Close()
			return
		}

		t.mu.Lock()
		t.seen += chunkLen
		t.mu.Unlock()

		select {
		case <-link.Done():
			return
		case out <- chunk:
		}
	}
}
