package toxics

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
)

// Direction defines the network stream direction.
type Direction string

const (
	Upstream   Direction = "upstream"
	Downstream Direction = "downstream"
)

// Chunk represents a discrete packet/buffer passing through the toxic pipeline.
type Chunk struct {
	Data      []byte
	Timestamp time.Time
}

// BufferPool maintains reusable byte slices to minimize GC pressure.
var BufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

// Link represents one half-duplex stream between client and upstream target.
type Link struct {
	Source net.Conn
	Target net.Conn
	Dir    Direction
	Stats  *telemetry.Stats

	closeOnce sync.Once
	closed    chan struct{}
}

// NewLink initializes a Link between two network sockets.
func NewLink(source, target net.Conn, dir Direction, stats *telemetry.Stats) *Link {
	return &Link{
		Source: source,
		Target: target,
		Dir:    dir,
		Stats:  stats,
		closed: make(chan struct{}),
	}
}

// Close closes both sockets and halts the link.
func (l *Link) Close() {
	l.closeOnce.Do(func() {
		close(l.closed)
		if l.Source != nil {
			_ = l.Source.Close()
		}
		if l.Target != nil {
			_ = l.Target.Close()
		}
	})
}

// Reset aborts the connection immediately by sending a TCP RST packet.
func (l *Link) Reset() {
	l.closeOnce.Do(func() {
		close(l.closed)
		for _, c := range []net.Conn{l.Source, l.Target} {
			if c == nil {
				continue
			}
			if tcp, ok := c.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
				_ = tcp.Close()
			} else {
				_ = c.Close()
			}
		}
	})
}

// IsClosed returns true if the link has been terminated.
func (l *Link) IsClosed() bool {
	select {
	case <-l.closed:
		return true
	default:
		return false
	}
}

// Done returns a channel signaling link closure.
func (l *Link) Done() <-chan struct{} {
	return l.closed
}

// Toxic defines the pluggable fault injection processor interface.
type Toxic interface {
	Name() string
	Type() string
	Stream() Direction
	Toxicity() float64
	Attributes() map[string]interface{}
	SetAttributes(attrs map[string]interface{}) error
	Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk)
	Clone() Toxic
}

// BaseToxic provides shared fields and helper logic across all toxic types.
type BaseToxic struct {
	ToxicName string                 `json:"name"`
	ToxicType string                 `json:"type"`
	ToxicDir  Direction              `json:"stream"`
	Prob      float64                `json:"toxicity"`
	Attrs     map[string]interface{} `json:"attributes"`
}

func (b *BaseToxic) Name() string {
	return b.ToxicName
}

func (b *BaseToxic) Type() string {
	return b.ToxicType
}

func (b *BaseToxic) Stream() Direction {
	return b.ToxicDir
}

func (b *BaseToxic) Toxicity() float64 {
	if b.Prob <= 0 {
		return 1.0
	}
	return b.Prob
}

func (b *BaseToxic) Attributes() map[string]interface{} {
	return b.Attrs
}

// ShouldApply evaluates whether a probabilistic toxic should execute on this cycle.
func (b *BaseToxic) ShouldApply() bool {
	if b.Prob >= 1.0 || b.Prob <= 0 {
		return true
	}
	var buf [8]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return true
	}
	val := float64(binary.BigEndian.Uint64(buf[:])) / float64(math.MaxUint64)
	return val <= b.Prob
}

// Constructor creates a new Toxic instance with default attributes.
type Constructor func(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Constructor)
)

// Register registers a new toxic type constructor.
func Register(toxicType string, c Constructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[toxicType] = c
}

// Create instantiates a Toxic by type name.
func Create(name, toxicType string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	registryMu.RLock()
	c, ok := registry[toxicType]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown toxic type: %s", toxicType)
	}

	if toxicity <= 0 {
		toxicity = 1.0
	}
	if dir == "" {
		dir = Downstream
	}
	return c(name, dir, toxicity, attrs)
}

// ErrInvalidAttribute indicates malformed configuration parameters.
var ErrInvalidAttribute = errors.New("invalid toxic attribute")
