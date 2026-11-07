package api

import (
	"encoding/json"
	"net/http"

	"github.com/alexandrmotologa/faultcraft/internal/presets"
	"github.com/alexandrmotologa/faultcraft/internal/proxy"
)

// handleListPresets returns all available chaos profiles.
func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(presets.GetPresets())
}

type ApplyPresetRequest struct {
	Proxy  string `json:"proxy"`
	Preset string `json:"preset"`
}

// handleApplyPreset activates a chaos scenario on a designated proxy.
func (s *Server) handleApplyPreset(w http.ResponseWriter, r *http.Request) {
	var req ApplyPresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"title":"invalid json","status":400}`, http.StatusBadRequest)
		return
	}

	p, ok := s.manager.Get(req.Proxy)
	if !ok {
		http.Error(w, fmtJSON("proxy not found", proxy.ErrProxyNotFound.Error(), 404), http.StatusNotFound)
		return
	}

	if err := presets.ApplyPreset(p, req.Preset); err != nil {
		http.Error(w, fmtJSON("failed applying preset", err.Error(), 400), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "applied",
		"proxy":   req.Proxy,
		"preset":  req.Preset,
		"details": makeProxyResponse(p),
	})
}
