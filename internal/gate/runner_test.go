package gate

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGateRunner(t *testing.T) {
	// Start mock upstream
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed starting mock upstream: %v", err)
	}
	defer ln.Close()

	tmpDir := t.TempDir()
	junitPath := filepath.Join(tmpDir, "report.xml")

	proxyDef := "test_gate=127.0.0.1:0->" + ln.Addr().String()

	opts := Options{
		ProxyDef:      proxyDef,
		Preset:        "flaky_db",
		MaxP99Latency: 5000 * time.Millisecond,
		MaxErrorRate:  0.50,
		ReportFormat:  "junit",
		ReportOutput:  junitPath,
		Command:       []string{"go", "version"},
	}

	res, err := Run(opts)
	if err != nil {
		t.Fatalf("gate run failed: %v", err)
	}

	if !res.Passed {
		t.Errorf("expected gate to pass, got violations: %v", res.Violations)
	}

	// Verify report exists and has content
	data, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatalf("failed to read generated junit report: %v", err)
	}

	if !bytes.Contains(data, []byte("<testsuites>")) {
		t.Errorf("expected XML testsuites in report, got: %s", string(data))
	}
}
