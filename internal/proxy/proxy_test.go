package proxy

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

// startMockEchoServer launches a TCP echo server on a loopback port.
func startMockEchoServer(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock echo server: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						_, _ = c.Write(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	return ln, ln.Addr().String()
}

func TestProxyDataForwarding(t *testing.T) {
	echoLn, echoAddr := startMockEchoServer(t)
	defer echoLn.Close()

	p := NewProxy("test_echo", "127.0.0.1:0", echoAddr)
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer p.Stop()

	client, err := net.DialTimeout("tcp", p.Listen, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer client.Close()

	payload := []byte("ping faultcraft engine")
	if _, err := client.Write(payload); err != nil {
		t.Fatalf("failed to send payload: %v", err)
	}

	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if !bytes.Equal(reply, payload) {
		t.Errorf("expected %q, got %q", payload, reply)
	}

	time.Sleep(50 * time.Millisecond)
	snap := p.Stats.Snapshot()
	if snap.BytesUpstream < uint64(len(payload)) {
		t.Errorf("expected at least %d bytes upstream, got %d", len(payload), snap.BytesUpstream)
	}
	if snap.BytesDownstream < uint64(len(payload)) {
		t.Errorf("expected at least %d bytes downstream, got %d", len(payload), snap.BytesDownstream)
	}
}

func TestProxyEnableDisable(t *testing.T) {
	echoLn, echoAddr := startMockEchoServer(t)
	defer echoLn.Close()

	p := NewProxy("test_toggle", "127.0.0.1:0", echoAddr)
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer p.Stop()

	listenAddr := p.Listen

	// Disable proxy
	if err := p.SetEnabled(false); err != nil {
		t.Fatalf("failed to disable proxy: %v", err)
	}

	// Dialing disabled proxy should fail
	conn, err := net.DialTimeout("tcp", listenAddr, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Errorf("expected dial to fail on disabled proxy")
	}

	// Re-enable proxy
	p.Listen = listenAddr
	if err := p.SetEnabled(true); err != nil {
		t.Fatalf("failed to re-enable proxy: %v", err)
	}

	client, err := net.DialTimeout("tcp", p.Listen, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial re-enabled proxy: %v", err)
	}
	_ = client.Close()
}

func TestProxyWithLatencyToxic(t *testing.T) {
	echoLn, echoAddr := startMockEchoServer(t)
	defer echoLn.Close()

	p := NewProxy("test_latency_proxy", "127.0.0.1:0", echoAddr)
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer p.Stop()

	// Attach 60ms latency toxic downstream
	latToxic, err := toxics.Create("down_delay", "latency", toxics.Downstream, 1.0, map[string]interface{}{
		"latency": 60,
		"jitter":  0,
	})
	if err != nil {
		t.Fatalf("failed to create toxic: %v", err)
	}

	if err := p.AddToxic(latToxic); err != nil {
		t.Fatalf("failed to add toxic: %v", err)
	}

	client, err := net.DialTimeout("tcp", p.Listen, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer client.Close()

	start := time.Now()
	payload := []byte("ping with latency")
	if _, err := client.Write(payload); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("failed to read reply: %v", err)
	}
	elapsed := time.Since(start)

	if !bytes.Equal(reply, payload) {
		t.Errorf("expected %q, got %q", payload, reply)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms total roundtrip with latency toxic, got %v", elapsed)
	}
}

func TestProxyHealthProber(t *testing.T) {
	echoLn, echoAddr := startMockEchoServer(t)

	p := NewProxy("test_prober", "127.0.0.1:0", echoAddr)
	h := p.ProbeTarget()

	if h.Status != "healthy" {
		t.Errorf("expected healthy status for active upstream, got %s (err: %s)", h.Status, h.Error)
	}
	if h.RTTMs < 0 {
		t.Errorf("expected non-negative RTT, got %f", h.RTTMs)
	}

	// Close upstream listener
	_ = echoLn.Close()
	time.Sleep(20 * time.Millisecond)

	hDown := p.ProbeTarget()
	if hDown.Status != "down" {
		t.Errorf("expected down status for closed upstream, got %s", hDown.Status)
	}
	if hDown.Error == "" {
		t.Errorf("expected error string when upstream is unreachable")
	}
}

func TestProxyAutoHealToxic(t *testing.T) {
	echoLn, echoAddr := startMockEchoServer(t)
	defer echoLn.Close()

	p := NewProxy("test_autoheal", "127.0.0.1:0", echoAddr)
	if err := p.Start(); err != nil {
		t.Fatalf("failed starting proxy: %v", err)
	}
	defer p.Stop()

	// Toxic with 80ms duration
	autoHealToxic, err := toxics.Create("short_delay", "latency", toxics.Downstream, 1.0, map[string]interface{}{
		"latency":  50,
		"duration": 80,
	})
	if err != nil {
		t.Fatalf("failed creating toxic: %v", err)
	}

	if err := p.AddToxic(autoHealToxic); err != nil {
		t.Fatalf("failed adding toxic: %v", err)
	}

	if _, exists := p.GetToxic("short_delay"); !exists {
		t.Fatalf("expected toxic to exist initially")
	}

	// Wait for auto-heal
	time.Sleep(120 * time.Millisecond)

	if _, exists := p.GetToxic("short_delay"); exists {
		t.Errorf("expected toxic to auto-heal and be removed after duration expired")
	}
}

func TestProxyManagerLifecycle(t *testing.T) {
	echoLn, echoAddr := startMockEchoServer(t)
	defer echoLn.Close()

	mgr := NewManager()

	p1 := NewProxy("proxy_1", "127.0.0.1:0", echoAddr)
	p2 := NewProxy("proxy_2", "127.0.0.1:0", echoAddr)

	if err := mgr.Add(p1); err != nil {
		t.Fatalf("failed to add p1: %v", err)
	}
	if err := mgr.Add(p2); err != nil {
		t.Fatalf("failed to add p2: %v", err)
	}

	if len(mgr.List()) != 2 {
		t.Errorf("expected 2 proxies, got %d", len(mgr.List()))
	}

	found, ok := mgr.Get("proxy_1")
	if !ok || found.Name != "proxy_1" {
		t.Errorf("expected to find proxy_1")
	}

	if err := mgr.Reset(); err != nil {
		t.Errorf("failed to reset proxies: %v", err)
	}

	if err := mgr.Delete("proxy_1"); err != nil {
		t.Errorf("failed to delete proxy_1: %v", err)
	}

	if len(mgr.List()) != 1 {
		t.Errorf("expected 1 proxy remaining, got %d", len(mgr.List()))
	}

	mgr.Clear()
	if len(mgr.List()) != 0 {
		t.Errorf("expected 0 proxies after Clear, got %d", len(mgr.List()))
	}
}
