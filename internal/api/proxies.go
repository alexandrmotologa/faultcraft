package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

// ProxyResponse represents the Toxiproxy v2 JSON serialization schema.
type ProxyResponse struct {
	Name         string             `json:"name"`
	Listen       string             `json:"listen"`
	Upstream     string             `json:"upstream"`
	Enabled      bool               `json:"enabled"`
	TargetHealth proxy.TargetHealth `json:"target_health"`
	Toxics       []toxics.BaseToxic `json:"toxics"`
}

func makeProxyResponse(p *proxy.Proxy) ProxyResponse {
	toxList := p.GetToxics()
	baseToxics := make([]toxics.BaseToxic, 0, len(toxList))
	for _, t := range toxList {
		baseToxics = append(baseToxics, toxics.BaseToxic{
			ToxicName:     t.Name(),
			ToxicType:     t.Type(),
			ToxicDir:      t.Stream(),
			Prob:          t.Toxicity(),
			Attrs:         t.Attributes(),
			DurationMs:    t.Duration(),
			ExpiresAtTime: t.ExpiresAt(),
		})
	}

	return ProxyResponse{
		Name:         p.Name,
		Listen:       p.Listen,
		Upstream:     p.Upstream,
		Enabled:      p.Enabled,
		TargetHealth: p.GetTargetHealth(),
		Toxics:       baseToxics,
	}
}

// handleListProxies returns a map of all proxies indexed by name.
func (s *Server) handleListProxies(w http.ResponseWriter, r *http.Request) {
	proxies := s.manager.List()
	resp := make(map[string]ProxyResponse, len(proxies))
	for _, p := range proxies {
		resp[p.Name] = makeProxyResponse(p)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type CreateProxyRequest struct {
	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  *bool  `json:"enabled"`
}

// handleCreateProxy registers and starts a new proxy.
func (s *Server) handleCreateProxy(w http.ResponseWriter, r *http.Request) {
	var req CreateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Listen == "" || req.Upstream == "" {
		http.Error(w, `{"title":"name, listen, and upstream are required","status":400}`, http.StatusBadRequest)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	p := proxy.NewProxy(req.Name, req.Listen, req.Upstream)
	p.Enabled = enabled

	if err := s.manager.Add(p); err != nil {
		if err == proxy.ErrProxyExists {
			http.Error(w, `{"title":"proxy already exists","status":409}`, http.StatusConflict)
		} else {
			http.Error(w, fmtJSON("failed to start proxy", err.Error(), 500), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(makeProxyResponse(p))
}

// handlePopulate batch creates proxies.
func (s *Server) handlePopulate(w http.ResponseWriter, r *http.Request) {
	var req []CreateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	created := make([]ProxyResponse, 0, len(req))
	for _, item := range req {
		p := proxy.NewProxy(item.Name, item.Listen, item.Upstream)
		if item.Enabled != nil {
			p.Enabled = *item.Enabled
		}
		if err := s.manager.Add(p); err != nil {
			http.Error(w, fmtJSON("populate error on "+item.Name, err.Error(), 400), http.StatusBadRequest)
			return
		}
		created = append(created, makeProxyResponse(p))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"proxies": created,
	})
}

// handleGetProxy returns a single proxy by name.
func (s *Server) handleGetProxy(w http.ResponseWriter, r *http.Request) {
	name := extractProxyName(r.URL.Path)
	p, ok := s.manager.Get(name)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(makeProxyResponse(p))
}

type UpdateProxyRequest struct {
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  *bool  `json:"enabled"`
}

// handleUpdateProxy updates proxy properties or toggles state.
func (s *Server) handleUpdateProxy(w http.ResponseWriter, r *http.Request) {
	name := extractProxyName(r.URL.Path)
	p, ok := s.manager.Get(name)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	var req UpdateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	if req.Enabled != nil {
		if err := p.SetEnabled(*req.Enabled); err != nil {
			http.Error(w, fmtJSON("failed setting enabled state", err.Error(), 500), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(makeProxyResponse(p))
}

// handleDeleteProxy stops and deletes a proxy.
func (s *Server) handleDeleteProxy(w http.ResponseWriter, r *http.Request) {
	name := extractProxyName(r.URL.Path)
	if err := s.manager.Delete(name); err != nil {
		if err == proxy.ErrProxyNotFound {
			http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		} else {
			http.Error(w, fmtJSON("failed deleting proxy", err.Error(), 500), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleReset resets all toxics and enables all proxies.
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if err := s.manager.Reset(); err != nil {
		http.Error(w, fmtJSON("reset failed", err.Error(), 500), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func extractProxyName(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/proxies/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func fmtJSON(title, message string, status int) string {
	b, _ := json.Marshal(map[string]interface{}{
		"title":   title,
		"message": message,
		"status":  status,
	})
	return string(b)
}
