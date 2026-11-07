package toxics

import (
	"sync"
	"time"
)

func init() {
	Register("bandwidth", NewBandwidthToxic)
}

// BandwidthToxic limits throughput to a specified rate in kilobytes per second.
type BandwidthToxic struct {
	BaseToxic

	mu            sync.Mutex
	Rate          int64 `json:"rate"` // Rate in KB/s
	nextAvailable time.Time
}

// NewBandwidthToxic constructs a BandwidthToxic.
func NewBandwidthToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &BandwidthToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "bandwidth",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		Rate: 100, // Default 100 KB/s
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *BandwidthToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if rate, ok := attrs["rate"]; ok {
		t.Rate = toInt64(rate)
	}
	return nil
}

func (t *BandwidthToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &BandwidthToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		Rate: t.Rate,
	}
}

func (t *BandwidthToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		if t.ShouldApply() && t.Rate > 0 {
			t.mu.Lock()
			bytesPerSec := t.Rate * 1024
			chunkBytes := int64(len(chunk.Data))
			duration := time.Duration(float64(chunkBytes) / float64(bytesPerSec) * float64(time.Second))

			now := time.Now()
			if t.nextAvailable.Before(now) {
				t.nextAvailable = now
			}
			t.nextAvailable = t.nextAvailable.Add(duration)
			sleepTime := time.Until(t.nextAvailable)
			t.mu.Unlock()

			if sleepTime > 0 {
				link.Stats.RecordFault()
				select {
				case <-link.Done():
					return
				case <-time.After(sleepTime):
				}
			}
		}

		select {
		case <-link.Done():
			return
		case out <- chunk:
		}
	}
}
