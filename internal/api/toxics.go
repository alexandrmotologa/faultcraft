package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

type ToxicRequest struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Stream     toxics.Direction       `json:"stream"`
	Toxicity   float64                `json:"toxicity"`
	Attributes map[string]interface{} `json:"attributes"`
}

// handleListToxics returns all toxics configured on a proxy.
func (s *Server) handleListToxics(w http.ResponseWriter, r *http.Request) {
	proxyName := extractProxyName(r.URL.Path)
	p, ok := s.manager.Get(proxyName)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	toxList := p.GetToxics()
	resp := make([]toxics.BaseToxic, 0, len(toxList))
	for _, t := range toxList {
		resp = append(resp, toxics.BaseToxic{
			ToxicName:     t.Name(),
			ToxicType:     t.Type(),
			ToxicDir:      t.Stream(),
			Prob:          t.Toxicity(),
			Attrs:         t.Attributes(),
			DurationMs:    t.Duration(),
			ExpiresAtTime: t.ExpiresAt(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleCreateToxic adds a new toxic to the proxy pipeline.
func (s *Server) handleCreateToxic(w http.ResponseWriter, r *http.Request) {
	proxyName := extractProxyName(r.URL.Path)
	p, ok := s.manager.Get(proxyName)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	var req ToxicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, `{"title":"toxic type is required","status":400}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = req.Type + "_" + string(req.Stream)
	}

	t, err := toxics.Create(req.Name, req.Type, req.Stream, req.Toxicity, req.Attributes)
	if err != nil {
		http.Error(w, fmtJSON("cannot create toxic", err.Error(), 400), http.StatusBadRequest)
		return
	}

	if err := p.AddToxic(t); err != nil {
		if err == proxy.ErrToxicExists {
			http.Error(w, `{"title":"toxic already exists","status":409}`, http.StatusConflict)
		} else {
			http.Error(w, fmtJSON("cannot add toxic", err.Error(), 500), http.StatusInternalServerError)
		}
		return
	}

	resp := toxics.BaseToxic{
		ToxicName:     t.Name(),
		ToxicType:     t.Type(),
		ToxicDir:      t.Stream(),
		Prob:          t.Toxicity(),
		Attrs:         t.Attributes(),
		DurationMs:    t.Duration(),
		ExpiresAtTime: t.ExpiresAt(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetToxic retrieves a specific toxic by name.
func (s *Server) handleGetToxic(w http.ResponseWriter, r *http.Request) {
	proxyName, toxicName := extractProxyAndToxicNames(r.URL.Path)
	p, ok := s.manager.Get(proxyName)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	t, found := p.GetToxic(toxicName)
	if !found {
		http.Error(w, `{"title":"toxic not found","status":404}`, http.StatusNotFound)
		return
	}

	resp := toxics.BaseToxic{
		ToxicName:     t.Name(),
		ToxicType:     t.Type(),
		ToxicDir:      t.Stream(),
		Prob:          t.Toxicity(),
		Attrs:         t.Attributes(),
		DurationMs:    t.Duration(),
		ExpiresAtTime: t.ExpiresAt(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleUpdateToxic updates attributes on an active toxic.
func (s *Server) handleUpdateToxic(w http.ResponseWriter, r *http.Request) {
	proxyName, toxicName := extractProxyAndToxicNames(r.URL.Path)
	p, ok := s.manager.Get(proxyName)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	var req ToxicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	t, err := p.UpdateToxic(toxicName, req.Attributes, req.Toxicity)
	if err != nil {
		if err == proxy.ErrToxicNotFound {
			http.Error(w, `{"title":"toxic not found","status":404}`, http.StatusNotFound)
		} else {
			http.Error(w, fmtJSON("cannot update toxic", err.Error(), 400), http.StatusBadRequest)
		}
		return
	}

	resp := toxics.BaseToxic{
		ToxicName:     t.Name(),
		ToxicType:     t.Type(),
		ToxicDir:      t.Stream(),
		Prob:          t.Toxicity(),
		Attrs:         t.Attributes(),
		DurationMs:    t.Duration(),
		ExpiresAtTime: t.ExpiresAt(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleDeleteToxic removes a toxic from the proxy pipeline.
func (s *Server) handleDeleteToxic(w http.ResponseWriter, r *http.Request) {
	proxyName, toxicName := extractProxyAndToxicNames(r.URL.Path)
	p, ok := s.manager.Get(proxyName)
	if !ok {
		http.Error(w, `{"title":"proxy not found","status":404}`, http.StatusNotFound)
		return
	}

	if err := p.RemoveToxic(toxicName); err != nil {
		if err == proxy.ErrToxicNotFound {
			http.Error(w, `{"title":"toxic not found","status":404}`, http.StatusNotFound)
		} else {
			http.Error(w, fmtJSON("cannot delete toxic", err.Error(), 500), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func extractProxyAndToxicNames(path string) (string, string) {
	// Path looks like /proxies/{proxyName}/toxics/{toxicName}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 4 && parts[0] == "proxies" && parts[2] == "toxics" {
		return parts[1], parts[3]
	}
	return "", ""
}
