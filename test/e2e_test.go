package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/api"
	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/web"
)

func TestEndToEndChaosLifecycle(t *testing.T) {
	// 1. Launch Mock Upstream TCP Echo Server
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed starting echo server: %v", err)
	}
	defer echoLn.Close()
	echoAddr := echoLn.Addr().String()

	go func() {
		for {
			conn, err := echoLn.Accept()
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

	// 2. Launch Mock Upstream HTTP Server
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed starting http server: %v", err)
	}
	defer httpLn.Close()
	httpAddr := httpLn.Addr().String()

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PONG"))
	})
	go func() {
		_ = http.Serve(httpLn, httpMux)
	}()

	// 3. Launch FaultCraft Daemon Server
	mgr := proxy.NewManager()
	defer mgr.Clear()

	srv := api.NewServer(mgr, web.GetFS(), "0.1.0-test")
	defer srv.Close()

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// 4. Verify Embedded Web UI is served at "/"
	uiResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("failed fetching UI: %v", err)
	}
	defer uiResp.Body.Close()
	if uiResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from web UI root, got %d", uiResp.StatusCode)
	}
	uiBody, _ := io.ReadAll(uiResp.Body)
	if !bytes.Contains(uiBody, []byte("FaultCraft")) {
		t.Errorf("expected 'FaultCraft' in embedded UI HTML, got: %s", string(uiBody))
	}

	// 5. Create TCP Proxy via REST API
	createPayload := fmt.Sprintf(`{"name":"e2e_echo","listen":"127.0.0.1:0","upstream":"%s"}`, echoAddr)
	resp, err := http.Post(ts.URL+"/proxies", "application/json", bytes.NewReader([]byte(createPayload)))
	if err != nil {
		t.Fatalf("POST /proxies failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var createdProxy api.ProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&createdProxy); err != nil {
		t.Fatalf("failed parsing created proxy response: %v", err)
	}
	proxyListenAddr := createdProxy.Listen

	// 6. Test Baseline TCP Traffic (Zero Interference)
	c, err := net.DialTimeout("tcp", proxyListenAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed connecting to proxy: %v", err)
	}

	start := time.Now()
	payload := []byte("deterministic resilience baseline")
	if _, err := c.Write(payload); err != nil {
		t.Fatalf("failed writing to proxy: %v", err)
	}

	recv := make([]byte, len(payload))
	if _, err := io.ReadFull(c, recv); err != nil {
		t.Fatalf("failed reading from proxy: %v", err)
	}
	baselineDuration := time.Since(start)
	_ = c.Close()

	if !bytes.Equal(recv, payload) {
		t.Errorf("expected %q, got %q", payload, recv)
	}
	if baselineDuration > 50*time.Millisecond {
		t.Errorf("baseline connection should be sub-50ms, took %v", baselineDuration)
	}

	// 7. Inject 75ms Latency Toxic via REST API
	toxicPayload := `{
		"name": "e2e_delay",
		"type": "latency",
		"stream": "downstream",
		"toxicity": 1.0,
		"attributes": {
			"latency": 75,
			"jitter": 0
		}
	}`
	resp, err = http.Post(ts.URL+"/proxies/e2e_echo/toxics", "application/json", bytes.NewReader([]byte(toxicPayload)))
	if err != nil {
		t.Fatalf("POST toxics failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from adding toxic, got %d", resp.StatusCode)
	}

	// 8. Assert Injected Latency Impact
	c, err = net.DialTimeout("tcp", proxyListenAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed connecting to proxy: %v", err)
	}

	start = time.Now()
	if _, err := c.Write(payload); err != nil {
		t.Fatalf("failed writing: %v", err)
	}
	if _, err := io.ReadFull(c, recv); err != nil {
		t.Fatalf("failed reading: %v", err)
	}
	delayedDuration := time.Since(start)
	_ = c.Close()

	if delayedDuration < 60*time.Millisecond {
		t.Errorf("expected injected delay of at least 60ms, took %v", delayedDuration)
	}

	// 9. Create HTTP Proxy and Inject HTTP Error Toxic (504 Gateway Timeout)
	httpProxyPayload := fmt.Sprintf(`{"name":"e2e_http","listen":"127.0.0.1:0","upstream":"%s"}`, httpAddr)
	resp, err = http.Post(ts.URL+"/proxies", "application/json", bytes.NewReader([]byte(httpProxyPayload)))
	if err != nil {
		t.Fatalf("POST /proxies for http failed: %v", err)
	}
	defer resp.Body.Close()
	var createdHTTPProxy api.ProxyResponse
	_ = json.NewDecoder(resp.Body).Decode(&createdHTTPProxy)

	httpToxicPayload := `{
		"name": "e2e_504",
		"type": "http_error",
		"stream": "downstream",
		"toxicity": 1.0,
		"attributes": {
			"status": 504,
			"body": "{\"error\":\"gateway_timeout\"}\n"
		}
	}`
	resp, err = http.Post(ts.URL+"/proxies/e2e_http/toxics", "application/json", bytes.NewReader([]byte(httpToxicPayload)))
	if err != nil {
		t.Fatalf("POST http toxic failed: %v", err)
	}
	defer resp.Body.Close()

	// Perform HTTP request through proxy
	httpReqClient := &http.Client{Timeout: 3 * time.Second}
	chaosHTTPResp, err := httpReqClient.Get("http://" + createdHTTPProxy.Listen + "/ping")
	if err != nil {
		t.Fatalf("GET through http chaos proxy failed: %v", err)
	}
	defer chaosHTTPResp.Body.Close()

	if chaosHTTPResp.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("expected HTTP 504 Gateway Timeout, got %d", chaosHTTPResp.StatusCode)
	}
	chaosBody, _ := io.ReadAll(chaosHTTPResp.Body)
	if !bytes.Contains(chaosBody, []byte("gateway_timeout")) {
		t.Errorf("expected rewritten body, got: %s", string(chaosBody))
	}

	// 10. Call /reset to clear all toxics
	resp, err = http.Post(ts.URL+"/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /reset failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from reset, got %d", resp.StatusCode)
	}

	// 11. Verify traffic is restored to normal after reset
	normalHTTPResp, err := httpReqClient.Get("http://" + createdHTTPProxy.Listen + "/ping")
	if err != nil {
		t.Fatalf("GET after reset failed: %v", err)
	}
	defer normalHTTPResp.Body.Close()
	if normalHTTPResp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP 200 OK after reset, got %d", normalHTTPResp.StatusCode)
	}

	// 12. Delete proxies
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/proxies/e2e_echo", nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil || delResp.StatusCode != http.StatusNoContent {
		t.Errorf("failed deleting proxy e2e_echo: %v", err)
	}

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/proxies/e2e_http", nil)
	delResp, err = http.DefaultClient.Do(req)
	if err != nil || delResp.StatusCode != http.StatusNoContent {
		t.Errorf("failed deleting proxy e2e_http: %v", err)
	}
}
