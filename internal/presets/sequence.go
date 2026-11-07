package presets

import (
	"fmt"
	"sync"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

// SequenceStep defines a single time-bounded phase in a chaos drill.
type SequenceStep struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	DurationSec int      `json:"duration_sec"`
	Toxics      []Def    `json:"toxics"`
}

// ChaosSequence defines an automated progression of chaos steps.
type ChaosSequence struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []SequenceStep `json:"steps"`
}

// BuiltinSequences provides standard incident simulation scenarios.
var BuiltinSequences = []ChaosSequence{
	{
		ID:          "db_failover_drill",
		Name:        "Database Failover & Reconnect Drill",
		Description: "Simulates database lock spikes, cascading socket timeouts, server crash, and automatic client recovery.",
		Steps: []SequenceStep{
			{
				Name:        "Phase 1: Baseline Warmup",
				Description: "Zero interference; allows application connection pools to stabilize.",
				DurationSec: 5,
				Toxics:      []Def{},
			},
			{
				Name:        "Phase 2: High Lock Contention",
				Description: "Injects +250ms Gaussian latency to test transaction timeouts.",
				DurationSec: 10,
				Toxics: []Def{
					{
						Name:     "seq_lat",
						Type:     "latency",
						Stream:   toxics.Downstream,
						Toxicity: 1.0,
						Attributes: map[string]interface{}{
							"latency": 250,
							"jitter":  40,
						},
					},
				},
			},
			{
				Name:        "Phase 3: Connection Resets & Throttling",
				Description: "Bandwidth throttling to 128 KB/s and TCP RST aborts to trigger client circuit breaker.",
				DurationSec: 10,
				Toxics: []Def{
					{
						Name:     "seq_bw",
						Type:     "bandwidth",
						Stream:   toxics.Downstream,
						Toxicity: 1.0,
						Attributes: map[string]interface{}{
							"rate": 128,
						},
					},
					{
						Name:     "seq_rst",
						Type:     "reset_peer",
						Stream:   toxics.Downstream,
						Toxicity: 0.15,
						Attributes: map[string]interface{}{
							"timeout": 1000,
						},
					},
				},
			},
			{
				Name:        "Phase 4: Dead Worker Hang",
				Description: "Unresponsive socket simulation testing dead socket reaping.",
				DurationSec: 8,
				Toxics: []Def{
					{
						Name:     "seq_hang",
						Type:     "hang",
						Stream:   toxics.Downstream,
						Toxicity: 1.0,
						Attributes: map[string]interface{}{},
					},
				},
			},
			{
				Name:        "Phase 5: Full Auto-Recovery",
				Description: "Clean connection restored; client should reconnect and resume operations.",
				DurationSec: 5,
				Toxics:      []Def{},
			},
		},
	},
	{
		ID:          "payment_cascade_drill",
		Name:        "Payment Gateway Outage & Fallback Drill",
		Description: "Simulates third-party payment vendor degradation: high latency, HTTP 504 errors, and fallback validation.",
		Steps: []SequenceStep{
			{
				Name:        "Step 1: Normal Operation",
				Description: "All payment requests execute normally.",
				DurationSec: 5,
				Toxics:      []Def{},
			},
			{
				Name:        "Step 2: Slow Payment Responses",
				Description: "Third-party payment vendor slows to 400ms delay.",
				DurationSec: 8,
				Toxics: []Def{
					{
						Name:     "pay_lat",
						Type:     "latency",
						Stream:   toxics.Downstream,
						Toxicity: 1.0,
						Attributes: map[string]interface{}{
							"latency": 400,
							"jitter":  50,
						},
					},
				},
			},
			{
				Name:        "Step 3: Total 504 Gateway Outage",
				Description: "100% of payment responses rewritten to HTTP 504 Gateway Timeout.",
				DurationSec: 10,
				Toxics: []Def{
					{
						Name:     "pay_504",
						Type:     "http_error",
						Stream:   toxics.Downstream,
						Toxicity: 1.0,
						Attributes: map[string]interface{}{
							"status": 504,
							"body":   "{\"error\":\"payment_timeout\"}\n",
						},
					},
				},
			},
			{
				Name:        "Step 4: Restored Vendor Service",
				Description: "Traffic returns to normal.",
				DurationSec: 5,
				Toxics:      []Def{},
			},
		},
	},
}

// SequenceStatus tracks live execution state of an automated timeline drill.
type SequenceStatus struct {
	SequenceID   string `json:"sequence_id"`
	ProxyName    string `json:"proxy_name"`
	CurrentStep  int    `json:"current_step"`
	TotalSteps   int    `json:"total_steps"`
	StepName     string `json:"step_name"`
	RemainingSec int    `json:"remaining_sec"`
	Running      bool   `json:"running"`
}

// SequenceRunner manages asynchronous sequence execution on target proxies.
type SequenceRunner struct {
	mu     sync.RWMutex
	status SequenceStatus
	stopCh chan struct{}
}

// GlobalRunner is the singleton sequence coordinator.
var GlobalRunner = &SequenceRunner{}

func (sr *SequenceRunner) GetStatus() SequenceStatus {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.status
}

func (sr *SequenceRunner) Stop(p *proxy.Proxy) {
	sr.mu.Lock()
	if sr.stopCh != nil {
		close(sr.stopCh)
		sr.stopCh = nil
	}
	sr.status = SequenceStatus{Running: false}
	sr.mu.Unlock()

	if p != nil {
		p.ResetToxics()
	}
}

func (sr *SequenceRunner) Run(p *proxy.Proxy, seqID string) error {
	var seq *ChaosSequence
	for i := range BuiltinSequences {
		if BuiltinSequences[i].ID == seqID {
			seq = &BuiltinSequences[i]
			break
		}
	}
	if seq == nil {
		return fmt.Errorf("sequence %q not found", seqID)
	}

	sr.Stop(p) // Ensure any previous sequence is stopped

	sr.mu.Lock()
	sr.stopCh = make(chan struct{})
	sr.status = SequenceStatus{
		SequenceID:   seq.ID,
		ProxyName:    p.Name,
		CurrentStep:  1,
		TotalSteps:   len(seq.Steps),
		StepName:     seq.Steps[0].Name,
		RemainingSec: seq.Steps[0].DurationSec,
		Running:      true,
	}
	stopChan := sr.stopCh
	sr.mu.Unlock()

	go func() {
		defer func() {
			sr.mu.Lock()
			sr.status.Running = false
			sr.mu.Unlock()
			p.ResetToxics()
		}()

		for stepIdx, step := range seq.Steps {
			// 1. Clear previous toxics
			p.ResetToxics()

			// 2. Attach new toxics for this phase
			for _, def := range step.Toxics {
				t, err := toxics.Create(def.Name, def.Type, def.Stream, def.Toxicity, def.Attributes)
				if err == nil {
					_ = p.AddToxic(t)
				}
			}

			// 3. Countdown loop for step duration
			for sec := step.DurationSec; sec > 0; sec-- {
				sr.mu.Lock()
				sr.status.CurrentStep = stepIdx + 1
				sr.status.StepName = step.Name
				sr.status.RemainingSec = sec
				sr.mu.Unlock()

				select {
				case <-stopChan:
					return
				case <-time.After(1 * time.Second):
				}
			}
		}
	}()

	return nil
}
