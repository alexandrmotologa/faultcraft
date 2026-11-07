package toxics

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

func init() {
	Register("latency", NewLatencyToxic)
}

// LatencyToxic delays packets using a Gaussian normal distribution with jitter.
type LatencyToxic struct {
	BaseToxic

	mu          sync.Mutex
	Latency     int64   `json:"latency"`     // Mean latency in milliseconds
	Jitter      int64   `json:"jitter"`      // Jitter (standard deviation) in milliseconds
	Correlation float64 `json:"correlation"` // Temporal correlation (0.0 to 1.0)
	lastDelay   int64
	rnd         *rand.Rand
}

// NewLatencyToxic constructs a LatencyToxic.
func NewLatencyToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &LatencyToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "latency",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *LatencyToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs

	if lat, ok := attrs["latency"]; ok {
		t.Latency = toInt64(lat)
	}
	if jit, ok := attrs["jitter"]; ok {
		t.Jitter = toInt64(jit)
	}
	if corr, ok := attrs["correlation"]; ok {
		t.Correlation = toFloat64(corr)
	}
	return nil
}

func (t *LatencyToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &LatencyToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		Latency:     t.Latency,
		Jitter:      t.Jitter,
		Correlation: t.Correlation,
		rnd:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (t *LatencyToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		if t.ShouldApply() {
			t.mu.Lock()
			delayMs := t.calculateDelay()
			t.mu.Unlock()

			if delayMs > 0 {
				link.Stats.RecordFault()
				select {
				case <-link.Done():
					return
				case <-time.After(time.Duration(delayMs) * time.Millisecond):
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

// calculateDelay generates a Gaussian-distributed delay value respecting correlation.
func (t *LatencyToxic) calculateDelay() int64 {
	var target float64
	if t.Jitter <= 0 {
		target = float64(t.Latency)
	} else {
		// Box-Muller transform for normal distribution
		norm := t.rnd.NormFloat64()
		target = float64(t.Latency) + norm*float64(t.Jitter)
	}

	if target < 0 {
		target = 0
	}

	if t.Correlation > 0 && t.lastDelay > 0 {
		target = (float64(t.lastDelay) * t.Correlation) + (target * (1.0 - t.Correlation))
	}

	res := int64(math.Round(target))
	t.lastDelay = res
	return res
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}
