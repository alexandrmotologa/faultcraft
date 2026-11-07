package telemetry

import (
	"testing"
	"time"
)

func TestInspectorProtocolDissection(t *testing.T) {
	// 1. HTTP GET Request
	proto, summary := DissectProtocol([]byte("GET /api/v1/orders HTTP/1.1\r\nHost: example.com\r\n"), "upstream")
	if proto != "HTTP/1.1" || summary != "GET /api/v1/orders HTTP/1.1" {
		t.Errorf("expected HTTP/1.1 GET, got %s: %s", proto, summary)
	}

	// 2. HTTP Response
	proto, summary = DissectProtocol([]byte("HTTP/1.1 504 Gateway Timeout\r\nContent-Length: 0\r\n"), "downstream")
	if proto != "HTTP/1.1" || summary != "HTTP/1.1 504 Gateway Timeout" {
		t.Errorf("expected HTTP/1.1 504, got %s: %s", proto, summary)
	}

	// 3. Redis PING
	proto, summary = DissectProtocol([]byte("PING\r\n"), "upstream")
	if proto != "Redis RESP" || summary != "PING" {
		t.Errorf("expected Redis RESP PING, got %s: %s", proto, summary)
	}

	// 4. Postgres Simple Query 'Q'
	// 'Q' (1 byte) + Length (4 bytes = 13: 4 len bytes + 8 chars + 1 null) + "SELECT 1\x00"
	pgData := []byte{'Q', 0x00, 0x00, 0x00, 0x0d, 'S', 'E', 'L', 'E', 'C', 'T', ' ', '1', 0x00}
	proto, summary = DissectProtocol(pgData, "upstream")
	if proto != "PostgreSQL" || summary != "SQL: SELECT 1" {
		t.Errorf("expected PostgreSQL SELECT 1, got %s: %s", proto, summary)
	}
}

func TestInspectorRingBuffer(t *testing.T) {
	inspector := NewFrameInspector(3)

	inspector.RecordFrame(FrameRecord{Timestamp: time.Now(), Summary: "frame 1"})
	inspector.RecordFrame(FrameRecord{Timestamp: time.Now(), Summary: "frame 2"})
	inspector.RecordFrame(FrameRecord{Timestamp: time.Now(), Summary: "frame 3"})
	inspector.RecordFrame(FrameRecord{Timestamp: time.Now(), Summary: "frame 4"}) // Should evict frame 1

	recent := inspector.GetRecent(10)
	if len(recent) != 3 {
		t.Fatalf("expected 3 frames in ring buffer, got %d", len(recent))
	}

	if recent[0].Summary != "frame 2" {
		t.Errorf("expected oldest remaining frame to be 'frame 2', got %s", recent[0].Summary)
	}
	if recent[2].Summary != "frame 4" {
		t.Errorf("expected newest frame to be 'frame 4', got %s", recent[2].Summary)
	}

	inspector.Clear()
	if len(inspector.GetRecent(10)) != 0 {
		t.Errorf("expected empty buffer after Clear")
	}
}
