package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/presets"
	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
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

	// Frame Inspector
	mux.HandleFunc("/api/v1/inspector", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(telemetry.GlobalInspector.GetRecent(50))
		case http.MethodDelete:
			telemetry.GlobalInspector.Clear()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Sequence Runner
	mux.HandleFunc("/api/v1/sequences", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(presets.BuiltinSequences)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/sequences/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Proxy      string `json:"proxy"`
			SequenceID string `json:"sequence_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
			return
		}
		p, ok := s.manager.Get(req.Proxy)
		if !ok {
			http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
			return
		}
		if err := presets.GlobalRunner.Run(p, req.SequenceID); err != nil {
			http.Error(w, fmtJSON("failed running sequence", err.Error(), 400), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "running", "sequence": req.SequenceID})
	})

	mux.HandleFunc("/api/v1/sequences/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		presets.GlobalRunner.Stop(nil)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	})

	mux.HandleFunc("/api/v1/sequences/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(presets.GlobalRunner.GetStatus())
	})

	// Config Export / Import
	mux.HandleFunc("/api/v1/config/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleConfigExport(w, r)
	})

	mux.HandleFunc("/api/v1/config/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleConfigImport(w, r)
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

type ConfigBundle struct {
	Version    string          `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Proxies    []ProxyResponse `json:"proxies"`
}

func (s *Server) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	proxies := s.manager.List()
	bundle := ConfigBundle{
		Version:    s.version,
		ExportedAt: time.Now().UTC(),
		Proxies:    make([]ProxyResponse, 0, len(proxies)),
	}

	for _, p := range proxies {
		bundle.Proxies = append(bundle.Proxies, makeProxyResponse(p))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=faultcraft-config.json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(bundle)
}

func (s *Server) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	var bundle ConfigBundle
	if err := json.NewDecoder(r.Body).Decode(&bundle); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	importedProxies := 0
	importedToxics := 0

	for _, pr := range bundle.Proxies {
		p, exists := s.manager.Get(pr.Name)
		if !exists {
			p = proxy.NewProxy(pr.Name, pr.Listen, pr.Upstream)
			p.Enabled = pr.Enabled
			if err := s.manager.Add(p); err != nil {
				continue
			}
			importedProxies++
		} else {
			_ = p.SetEnabled(pr.Enabled)
		}

		p.ResetToxics()
		for _, tox := range pr.Toxics {
			t, err := toxics.Create(tox.ToxicName, tox.ToxicType, tox.ToxicDir, tox.Prob, tox.Attrs)
			if err == nil {
				if tox.DurationMs > 0 {
					if bt, ok := t.(interface{ SetDuration(int64) }); ok {
						bt.SetDuration(tox.DurationMs)
					}
				}
				_ = p.AddToxic(t)
				importedToxics++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "imported",
		"imported_proxies": importedProxies,
		"imported_toxics":  importedToxics,
	})
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
