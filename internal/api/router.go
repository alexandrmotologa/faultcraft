package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
)

// Server coordinates HTTP API routing and serves the embedded web cockpit.
type Server struct {
	manager *proxy.Manager
	broker  *SSEBroker
	uiFS    fs.FS
	version string
}

// NewServer initializes the API server with CORS and SSE capabilities.
func NewServer(mgr *proxy.Manager, uiFS fs.FS, version string) *Server {
	if version == "" {
		version = "0.1.0"
	}
	return &Server{
		manager: mgr,
		broker:  NewSSEBroker(mgr),
		uiFS:    uiFS,
		version: version,
	}
}

// Close cleans up background broadcaster resources.
func (s *Server) Close() {
	if s.broker != nil {
		s.broker.Close()
	}
}

// Router returns an http.Handler with all registered endpoints and middleware.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Version
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":     s.version,
			"daemon":      "faultcraft",
			"compatibility": "toxiproxy-v2",
		})
	})

	// Toxiproxy endpoints
	mux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.handleReset(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/populate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.handlePopulate(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleListProxies(w, r)
		case http.MethodPost:
			s.handleCreateProxy(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Dispatcher for /proxies/... paths
	mux.HandleFunc("/proxies/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")

		// Case 1: /proxies/{name} (len == 2)
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				s.handleGetProxy(w, r)
			case http.MethodPost:
				s.handleUpdateProxy(w, r)
			case http.MethodDelete:
				s.handleDeleteProxy(w, r)
			default:
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Case 2: /proxies/{name}/toxics (len == 3 && parts[2] == "toxics")
		if len(parts) == 3 && parts[2] == "toxics" {
			switch r.Method {
			case http.MethodGet:
				s.handleListToxics(w, r)
			case http.MethodPost:
				s.handleCreateToxic(w, r)
			default:
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Case 3: /proxies/{name}/toxics/{toxic_name} (len == 4 && parts[2] == "toxics")
		if len(parts) == 4 && parts[2] == "toxics" {
			switch r.Method {
			case http.MethodGet:
				s.handleGetToxic(w, r)
			case http.MethodPost:
				s.handleUpdateToxic(w, r)
			case http.MethodDelete:
				s.handleDeleteToxic(w, r)
			default:
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		http.NotFound(w, r)
	})

	// Extended FaultCraft endpoints
	mux.HandleFunc("/api/v1/presets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleListPresets(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/presets/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.handleApplyPreset(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Real-time SSE
	mux.Handle("/api/v1/events", s.broker)

	// Embedded Web UI Handler
	if s.uiFS != nil {
		fileServer := http.FileServer(http.FS(s.uiFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Don't intercept API routes
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/proxies") || r.URL.Path == "/version" || r.URL.Path == "/reset" {
				http.NotFound(w, r)
				return
			}

			// SPA Fallback: Check if file exists in uiFS, otherwise serve index.html
			filePath := strings.TrimPrefix(r.URL.Path, "/")
			if filePath == "" {
				filePath = "index.html"
			}

			f, err := s.uiFS.Open(filePath)
			if err != nil {
				// Fallback to index.html for client-side routing
				r.URL.Path = "/"
			} else {
				_ = f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
