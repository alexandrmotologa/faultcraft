package telemetry

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"
)

// FrameRecord represents a captured and dissected network frame.
type FrameRecord struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	ProxyName     string    `json:"proxy_name"`
	Direction     string    `json:"direction"` // "upstream" or "downstream"
	Protocol      string    `json:"protocol"`  // "HTTP/1.1", "Redis RESP", "PostgreSQL", "TCP"
	Summary       string    `json:"summary"`
	Bytes         int       `json:"bytes"`
	FaultInjected string    `json:"fault_injected,omitempty"`
	LatencyMs     float64   `json:"latency_ms"`
}

// FrameInspector maintains a concurrent in-memory rolling ring buffer of captured frames.
type FrameInspector struct {
	mu       sync.RWMutex
	records  []FrameRecord
	capacity int
	counter  uint64
}

// GlobalInspector is the shared system-wide inspector instance.
var GlobalInspector = NewFrameInspector(100)

// NewFrameInspector creates an inspector with a fixed ring buffer capacity.
func NewFrameInspector(capacity int) *FrameInspector {
	if capacity <= 0 {
		capacity = 100
	}
	return &FrameInspector{
		records:  make([]FrameRecord, 0, capacity),
		capacity: capacity,
	}
}

// RecordFrame stores a newly dissected frame in the ring buffer.
func (fi *FrameInspector) RecordFrame(rec FrameRecord) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.counter++
	rec.ID = fmt.Sprintf("frame_%d_%d", rec.Timestamp.UnixNano(), fi.counter)

	if len(fi.records) >= fi.capacity {
		// Evict oldest
		fi.records = fi.records[1:]
	}
	fi.records = append(fi.records, rec)
}

// GetRecent returns the most recent captured frames up to limit.
func (fi *FrameInspector) GetRecent(limit int) []FrameRecord {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	n := len(fi.records)
	if limit <= 0 || limit > n {
		limit = n
	}

	result := make([]FrameRecord, limit)
	copy(result, fi.records[n-limit:])
	return result
}

// Clear empties the ring buffer.
func (fi *FrameInspector) Clear() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.records = fi.records[:0]
}

// DissectProtocol analyzes raw bytes to identify L7 protocol structure and summary text.
func DissectProtocol(data []byte, dir string) (protocol string, summary string) {
	if len(data) == 0 {
		return "TCP", "Empty Frame"
	}

	// 1. Detect HTTP/1.1
	if bytes.HasPrefix(data, []byte("GET ")) ||
		bytes.HasPrefix(data, []byte("POST ")) ||
		bytes.HasPrefix(data, []byte("PUT ")) ||
		bytes.HasPrefix(data, []byte("DELETE ")) ||
		bytes.HasPrefix(data, []byte("PATCH ")) ||
		bytes.HasPrefix(data, []byte("HEAD ")) ||
		bytes.HasPrefix(data, []byte("OPTIONS ")) {
		line := readFirstLine(data)
		return "HTTP/1.1", line
	}

	if bytes.HasPrefix(data, []byte("HTTP/1.0 ")) || bytes.HasPrefix(data, []byte("HTTP/1.1 ")) {
		line := readFirstLine(data)
		return "HTTP/1.1", line
	}

	// 2. Detect Redis RESP (REdis Serialization Protocol)
	if bytes.HasPrefix(data, []byte("+PONG")) || bytes.HasPrefix(data, []byte("+OK")) {
		return "Redis RESP", readFirstLine(data)
	}
	if bytes.HasPrefix(data, []byte("PING")) || strings.EqualFold(string(bytes.TrimSpace(data)), "PING") {
		return "Redis RESP", "PING"
	}
	if len(data) > 1 && data[0] == '*' && data[1] >= '0' && data[1] <= '9' {
		// Redis Array Command
		cmd := parseRedisCommand(data)
		if cmd != "" {
			return "Redis RESP", cmd
		}
	}

	// 3. Detect PostgreSQL Wire Protocol Simple Query ('Q')
	if len(data) > 5 && data[0] == 'Q' {
		msgLen := int(binary.BigEndian.Uint32(data[1:5]))
		if msgLen >= 4 && 1+msgLen <= len(data) {
			query := string(bytes.TrimRight(data[5:1+msgLen], "\x00"))
			return "PostgreSQL", "SQL: " + truncate(query, 64)
		}
	}

	// 4. Default Raw TCP Stream Summary
	printable := toPrintable(data, 48)
	return "TCP", printable
}

func readFirstLine(data []byte) string {
	idx := bytes.IndexByte(data, '\n')
	if idx > 0 {
		return strings.TrimSpace(string(data[:idx]))
	}
	return truncate(string(data), 64)
}

func parseRedisCommand(data []byte) string {
	parts := strings.Split(string(data), "\r\n")
	var words []string
	for i := 1; i < len(parts); i++ {
		if strings.HasPrefix(parts[i], "$") {
			continue // Length line
		}
		if parts[i] != "" {
			words = append(words, parts[i])
			if len(words) >= 4 {
				break
			}
		}
	}
	if len(words) > 0 {
		return strings.Join(words, " ")
	}
	return "RESP Command"
}

func toPrintable(data []byte, maxLen int) string {
	var sb strings.Builder
	for i, b := range data {
		if i >= maxLen {
			sb.WriteString("...")
			break
		}
		if b >= 32 && b <= 126 {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("\\x%02x", b))
		}
	}
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
