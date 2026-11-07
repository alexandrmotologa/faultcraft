package toxics

import (
	"math/rand"
	"sync"
	"time"
)

func init() {
	Register("slicer", NewSlicerToxic)
}

// SlicerToxic breaks chunks into small packet fragments with microsecond delays.
type SlicerToxic struct {
	BaseToxic

	mu            sync.Mutex
	AverageSize   int64 `json:"average_size"`   // Target chunk slice size in bytes
	SizeVariation int64 `json:"size_variation"` // Variation range around average size
	Delay         int64 `json:"delay"`          // Delay between slices in microseconds
	rnd           *rand.Rand
}

// NewSlicerToxic constructs a SlicerToxic.
func NewSlicerToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &SlicerToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "slicer",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		AverageSize:   128,
		SizeVariation: 32,
		Delay:         1000, // 1000 microseconds = 1ms
		rnd:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *SlicerToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if size, ok := attrs["average_size"]; ok {
		t.AverageSize = toInt64(size)
	}
	if varSize, ok := attrs["size_variation"]; ok {
		t.SizeVariation = toInt64(varSize)
	}
	if d, ok := attrs["delay"]; ok {
		t.Delay = toInt64(d)
	}
	if t.AverageSize <= 0 {
		t.AverageSize = 128
	}
	return nil
}

func (t *SlicerToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &SlicerToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		AverageSize:   t.AverageSize,
		SizeVariation: t.SizeVariation,
		Delay:         t.Delay,
		rnd:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (t *SlicerToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		if !t.ShouldApply() || len(chunk.Data) <= int(t.AverageSize) {
			select {
			case <-link.Done():
				return
			case out <- chunk:
			}
			continue
		}

		link.Stats.RecordFault()
		offset := 0
		total := len(chunk.Data)

		for offset < total {
			if link.IsClosed() {
				return
			}

			t.mu.Lock()
			sliceSize := int(t.AverageSize)
			if t.SizeVariation > 0 {
				delta := t.rnd.Int63n(2*t.SizeVariation+1) - t.SizeVariation
				sliceSize += int(delta)
			}
			delayMicros := t.Delay
			t.mu.Unlock()

			if sliceSize <= 0 {
				sliceSize = 1
			}

			remaining := total - offset
			if sliceSize > remaining {
				sliceSize = remaining
			}

			subData := make([]byte, sliceSize)
			copy(subData, chunk.Data[offset:offset+sliceSize])
			offset += sliceSize

			select {
			case <-link.Done():
				return
			case out <- &Chunk{
				Data:      subData,
				Timestamp: chunk.Timestamp,
			}:
			}

			if delayMicros > 0 && offset < total {
				select {
				case <-link.Done():
					return
				case <-time.After(time.Duration(delayMicros) * time.Microsecond):
				}
			}
		}
	}
}
