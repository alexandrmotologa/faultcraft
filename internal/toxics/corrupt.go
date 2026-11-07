package toxics

import (
	"math/rand"
	"sync"
	"time"
)

func init() {
	Register("corrupt", NewCorruptToxic)
}

// CorruptToxic alters random bytes in the payload to test network protocol parser resilience.
type CorruptToxic struct {
	BaseToxic

	mu        sync.Mutex
	ByteIndex int64 `json:"byte_index"` // Specific byte index to tamper with, or -1 for random
	rnd       *rand.Rand
}

// NewCorruptToxic constructs a CorruptToxic.
func NewCorruptToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &CorruptToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "corrupt",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		ByteIndex: -1,
		rnd:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *CorruptToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if idx, ok := attrs["byte_index"]; ok {
		t.ByteIndex = toInt64(idx)
	}
	return nil
}

func (t *CorruptToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &CorruptToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		ByteIndex: t.ByteIndex,
		rnd:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (t *CorruptToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		if t.ShouldApply() && len(chunk.Data) > 0 {
			t.mu.Lock()
			idx := t.ByteIndex
			if idx < 0 || idx >= int64(len(chunk.Data)) {
				idx = t.rnd.Int63n(int64(len(chunk.Data)))
			}
			t.mu.Unlock()

			link.Stats.RecordFault()

			mutatedData := make([]byte, len(chunk.Data))
			copy(mutatedData, chunk.Data)
			// Flip all bits in the chosen byte
			mutatedData[idx] ^= 0xFF

			subChunk := &Chunk{
				Data:      mutatedData,
				Timestamp: chunk.Timestamp,
			}

			select {
			case <-link.Done():
				return
			case out <- subChunk:
			}
			continue
		}

		select {
		case <-link.Done():
			return
		case out <- chunk:
		}
	}
}
