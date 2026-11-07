package toxics

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
)

func TestToxicRegistry(t *testing.T) {
	types := []string{
		"latency", "bandwidth", "slicer", "timeout",
		"hang", "reset_peer", "limit_data", "http_error", "corrupt", "circuit_breaker_probe",
	}

	for _, tt := range types {
		toxic, err := Create("test_"+tt, tt, Downstream, 1.0, map[string]interface{}{})
		if err != nil {
			t.Fatalf("failed to create toxic %s: %v", tt, err)
		}
		if toxic.Type() != tt {
			t.Errorf("expected type %s, got %s", tt, toxic.Type())
		}
	}
}

func TestLatencyToxic(t *testing.T) {
	lat, err := Create("test_lat", "latency", Downstream, 1.0, map[string]interface{}{
		"latency": 50,
		"jitter":  5,
	})
	if err != nil {
		t.Fatalf("failed to create latency toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 1)
	out := make(chan *Chunk, 1)

	go lat.Pipe(link, in, out)

	start := time.Now()
	in <- &Chunk{Data: []byte("payload"), Timestamp: time.Now()}
	close(in)

	chunk := <-out
	elapsed := time.Since(start)

	if chunk == nil {
		t.Fatalf("expected chunk, got nil")
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected delay of at least 40ms, got %v", elapsed)
	}
}

func TestBandwidthToxic(t *testing.T) {
	bw, err := Create("test_bw", "bandwidth", Downstream, 1.0, map[string]interface{}{
		"rate": 10, // 10 KB/s = 10240 bytes/sec
	})
	if err != nil {
		t.Fatalf("failed to create bandwidth toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 1)
	out := make(chan *Chunk, 1)

	go bw.Pipe(link, in, out)

	data := make([]byte, 10240/2) // 5KB should take ~500ms at 10KB/s
	start := time.Now()
	in <- &Chunk{Data: data, Timestamp: time.Now()}
	close(in)

	chunk := <-out
	elapsed := time.Since(start)

	if chunk == nil {
		t.Fatalf("expected chunk, got nil")
	}
	if elapsed < 400*time.Millisecond {
		t.Errorf("expected bandwidth throttling of ~500ms, got %v", elapsed)
	}
}

func TestSlicerToxic(t *testing.T) {
	slicer, err := Create("test_slicer", "slicer", Downstream, 1.0, map[string]interface{}{
		"average_size":   10,
		"size_variation": 0,
		"delay":          0,
	})
	if err != nil {
		t.Fatalf("failed to create slicer toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 1)
	out := make(chan *Chunk, 10)

	go slicer.Pipe(link, in, out)

	in <- &Chunk{Data: make([]byte, 30), Timestamp: time.Now()}
	close(in)

	count := 0
	for chunk := range out {
		if len(chunk.Data) != 10 {
			t.Errorf("expected sliced chunk of 10 bytes, got %d", len(chunk.Data))
		}
		count++
	}

	if count != 3 {
		t.Errorf("expected 3 sliced chunks, got %d", count)
	}
}

func TestHTTPErrorToxic(t *testing.T) {
	httpToxic, err := Create("test_http", "http_error", Downstream, 1.0, map[string]interface{}{
		"status": 504,
		"body":   "{\"error\":\"gateway_timeout\"}",
	})
	if err != nil {
		t.Fatalf("failed to create http_error toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 1)
	out := make(chan *Chunk, 1)

	go httpToxic.Pipe(link, in, out)

	rawHTTP := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nSuccess")
	in <- &Chunk{Data: rawHTTP, Timestamp: time.Now()}
	close(in)

	chunk := <-out
	if chunk == nil {
		t.Fatalf("expected chunk, got nil")
	}

	if !bytes.Contains(chunk.Data, []byte("504 Gateway Timeout")) {
		t.Errorf("expected 504 Gateway Timeout in rewritten response, got:\n%s", string(chunk.Data))
	}
	if !bytes.Contains(chunk.Data, []byte("gateway_timeout")) {
		t.Errorf("expected custom body in rewritten response, got:\n%s", string(chunk.Data))
	}
}

func TestLimitDataToxic(t *testing.T) {
	limitToxic, err := Create("test_limit", "limit_data", Downstream, 1.0, map[string]interface{}{
		"bytes": 15,
	})
	if err != nil {
		t.Fatalf("failed to create limit_data toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 1)
	out := make(chan *Chunk, 2)

	go limitToxic.Pipe(link, in, out)

	in <- &Chunk{Data: make([]byte, 20), Timestamp: time.Now()}
	close(in)

	var totalBytes int
	for chunk := range out {
		totalBytes += len(chunk.Data)
	}

	if totalBytes != 15 {
		t.Errorf("expected 15 bytes delivered before cutoff, got %d", totalBytes)
	}
}

func TestCircuitBreakerToxic(t *testing.T) {
	cb, err := Create("test_cb", "circuit_breaker_probe", Downstream, 1.0, map[string]interface{}{
		"trip_after_errors": 2,
		"cooldown_ms":       50,
		"probe_successes":   1,
		"status_code":       503,
	})
	if err != nil {
		t.Fatalf("failed creating circuit_breaker_probe toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 5)
	out := make(chan *Chunk, 5)

	go cb.Pipe(link, in, out)

	rawHTTP := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK")

	// 1st request -> should fail with 503
	in <- &Chunk{Data: rawHTTP, Timestamp: time.Now()}
	c1 := <-out
	if !bytes.Contains(c1.Data, []byte("503 Service Unavailable")) {
		t.Errorf("expected 1st request to fail with 503, got: %s", string(c1.Data))
	}

	// 2nd request -> should fail with 503 (tripping the breaker to Open)
	in <- &Chunk{Data: rawHTTP, Timestamp: time.Now()}
	c2 := <-out
	if !bytes.Contains(c2.Data, []byte("503 Service Unavailable")) {
		t.Errorf("expected 2nd request to fail with 503, got: %s", string(c2.Data))
	}

	// Wait for cooldown to transition Open -> HalfOpen
	time.Sleep(70 * time.Millisecond)

	// 3rd request -> probe request in HalfOpen, should pass through with 200 OK
	in <- &Chunk{Data: rawHTTP, Timestamp: time.Now()}
	c3 := <-out
	if !bytes.Contains(c3.Data, []byte("200 OK")) {
		t.Errorf("expected probe request in half_open to succeed with 200 OK, got: %s", string(c3.Data))
	}
}

func TestCorruptToxic(t *testing.T) {
	corruptToxic, err := Create("test_corrupt", "corrupt", Downstream, 1.0, map[string]interface{}{
		"byte_index": 2,
	})
	if err != nil {
		t.Fatalf("failed to create corrupt toxic: %v", err)
	}

	stats := telemetry.NewStats()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	link := NewLink("test_proxy", client, server, Downstream, stats)
	defer link.Close()

	in := make(chan *Chunk, 1)
	out := make(chan *Chunk, 1)

	go corruptToxic.Pipe(link, in, out)

	orig := []byte{0x00, 0x00, 0x00, 0x00}
	in <- &Chunk{Data: orig, Timestamp: time.Now()}
	close(in)

	chunk := <-out
	if chunk == nil {
		t.Fatalf("expected chunk, got nil")
	}

	if chunk.Data[2] != 0xFF {
		t.Errorf("expected byte at index 2 to be corrupted to 0xFF, got %x", chunk.Data[2])
	}
}
