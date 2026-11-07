package presets

import (
	"fmt"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

// Preset represents a pre-configured chaos experiment profile.
type Preset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Toxics      []Def    `json:"toxics"`
}

// Def describes a single toxic declaration within a preset.
type Def struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Stream     toxics.Direction       `json:"stream"`
	Toxicity   float64                `json:"toxicity"`
	Attributes map[string]interface{} `json:"attributes"`
}

// BuiltinPresets provides production-grade simulated failure scenarios.
var BuiltinPresets = []Preset{
	{
		ID:          "flaky_db",
		Name:        "Flaky Database",
		Description: "High database lock contention with 180ms latency (+/- 40ms) and 3% intermittent TCP resets.",
		Category:    "Databases",
		Toxics: []Def{
			{
				Name:     "db_latency",
				Type:     "latency",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"latency": 180,
					"jitter":  40,
				},
			},
			{
				Name:     "db_resets",
				Type:     "reset_peer",
				Stream:   toxics.Downstream,
				Toxicity: 0.03,
				Attributes: map[string]interface{}{
					"timeout": 2000,
				},
			},
		},
	},
	{
		ID:          "cellular_3g",
		Name:        "Degraded 3G Mobile",
		Description: "Cellular network throttled to 256 KB/s with 350ms delay and 256-byte packet fragmentation.",
		Category:    "Network",
		Toxics: []Def{
			{
				Name:     "mobile_bandwidth",
				Type:     "bandwidth",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"rate": 256,
				},
			},
			{
				Name:     "mobile_latency",
				Type:     "latency",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"latency": 350,
					"jitter":  80,
				},
			},
			{
				Name:     "mobile_slicer",
				Type:     "slicer",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"average_size": 256,
					"delay":        2000,
				},
			},
		},
	},
	{
		ID:          "payment_outage",
		Name:        "Payment Gateway Outage",
		Description: "Third-party payment vendor outage returning HTTP 504 Gateway Timeout on 100% of requests.",
		Category:    "HTTP APIs",
		Toxics: []Def{
			{
				Name:     "gateway_timeout",
				Type:     "http_error",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"status": 504,
					"body":   "{\"error\":\"payment_gateway_timeout\",\"code\":504,\"retry_after\":30}\n",
				},
			},
		},
	},
	{
		ID:          "zombie_worker",
		Name:        "Zombie Socket Hang",
		Description: "Accepts TCP handshake but never transmits replies, testing client connection pool timeouts.",
		Category:    "Deadlock",
		Toxics: []Def{
			{
				Name:     "zombie_hang",
				Type:     "hang",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{},
			},
		},
	},
	{
		ID:          "packet_shredder",
		Name:        "Packet Shredder",
		Description: "Heavy packet slicing into 64-byte chunks with 100ms jitter and 1% payload bit corruption.",
		Category:    "Stress",
		Toxics: []Def{
			{
				Name:     "shred_slicer",
				Type:     "slicer",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"average_size": 64,
					"delay":        1000,
				},
			},
			{
				Name:     "shred_latency",
				Type:     "latency",
				Stream:   toxics.Downstream,
				Toxicity: 1.0,
				Attributes: map[string]interface{}{
					"latency": 100,
					"jitter":  50,
				},
			},
			{
				Name:     "shred_corrupt",
				Type:     "corrupt",
				Stream:   toxics.Downstream,
				Toxicity: 0.01,
				Attributes: map[string]interface{}{
					"byte_index": -1,
				},
			},
		},
	},
}

// GetPresets returns all available built-in chaos scenarios.
func GetPresets() []Preset {
	return BuiltinPresets
}

// FindPreset retrieves a preset by ID.
func FindPreset(id string) (*Preset, bool) {
	for i := range BuiltinPresets {
		if BuiltinPresets[i].ID == id {
			return &BuiltinPresets[i], true
		}
	}
	return nil, false
}

// ApplyPreset applies a preset to a designated proxy, clearing previous toxics.
func ApplyPreset(p *proxy.Proxy, presetID string) error {
	preset, found := FindPreset(presetID)
	if !found {
		return fmt.Errorf("preset %q not found", presetID)
	}

	p.ResetToxics()

	for _, def := range preset.Toxics {
		t, err := toxics.Create(def.Name, def.Type, def.Stream, def.Toxicity, def.Attributes)
		if err != nil {
			return fmt.Errorf("failed creating toxic %s: %w", def.Name, err)
		}
		if err := p.AddToxic(t); err != nil {
			return fmt.Errorf("failed adding toxic %s: %w", def.Name, err)
		}
	}
	return nil
}
