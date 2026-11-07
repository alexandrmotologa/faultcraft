package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
)

func TestAPIRoutes(t *testing.T) {
	// Start a mock upstream server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock upstream: %v", err)
	}
	defer ln.Close()

	mgr := proxy.NewManager()
	srv := NewServer(mgr, nil, "1.0.0")
	defer srv.Close()

	router := srv.Router()

	// 1. GET /version
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /version, got %d", w.Code)
	}

	// 2. POST /proxies (create)
	createBody := []byte(`{"name":"test_pg","listen":"127.0.0.1:0","upstream":"` + ln.Addr().String() + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/proxies", bytes.NewReader(createBody))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from POST /proxies, got %d (body: %s)", w.Code, w.Body.String())
	}

	// 3. GET /proxies
	req = httptest.NewRequest(http.MethodGet, "/proxies", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK from GET /proxies, got %d", w.Code)
	}

	var proxiesMap map[string]ProxyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &proxiesMap); err != nil {
		t.Fatalf("failed to parse proxies JSON: %v", err)
	}
	if _, ok := proxiesMap["test_pg"]; !ok {
		t.Errorf("expected test_pg in proxies list")
	}

	// 4. POST /proxies/test_pg/toxics
	toxicBody := []byte(`{
		"name": "db_delay",
		"type": "latency",
		"stream": "downstream",
		"toxicity": 1.0,
		"attributes": {
			"latency": 150,
			"jitter": 20
		}
	}`)
	req = httptest.NewRequest(http.MethodPost, "/proxies/test_pg/toxics", bytes.NewReader(toxicBody))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from POST /proxies/test_pg/toxics, got %d (body: %s)", w.Code, w.Body.String())
	}

	// 5. GET /proxies/test_pg/toxics
	req = httptest.NewRequest(http.MethodGet, "/proxies/test_pg/toxics", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK from GET toxics, got %d", w.Code)
	}

	// 6. POST /api/v1/presets/apply
	presetBody := []byte(`{"proxy":"test_pg","preset":"cellular_3g"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/presets/apply", bytes.NewReader(presetBody))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from apply preset, got %d (body: %s)", w.Code, w.Body.String())
	}

	// 7. POST /reset
	req = httptest.NewRequest(http.MethodPost, "/reset", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK from POST /reset, got %d", w.Code)
	}

	// 8. DELETE /proxies/test_pg
	req = httptest.NewRequest(http.MethodDelete, "/proxies/test_pg", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content from DELETE /proxies/test_pg, got %d", w.Code)
	}
}
