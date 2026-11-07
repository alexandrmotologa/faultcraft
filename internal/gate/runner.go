package gate

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/presets"
	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
)

// Options defines configuration parameters for a CI/CD chaos test run.
type Options struct {
	ProxyDef      string        // Format: name=listen->upstream
	Preset        string        // Builtin preset ID
	MaxP99Latency time.Duration // SLO threshold for p99 latency
	MaxErrorRate  float64       // SLO threshold for error rate (0.0 to 1.0)
	ReportFormat  string        // "console", "json", "junit"
	ReportOutput  string        // Output file path (empty = stdout)
	Command       []string      // Command line to execute
}

// Result captures the outcome and telemetry of a gated test execution.
type Result struct {
	Passed          bool               `json:"passed"`
	CommandExitCode int                `json:"command_exit_code"`
	DurationMs      int64              `json:"duration_ms"`
	Preset          string             `json:"preset"`
	Telemetry       telemetry.Snapshot `json:"telemetry"`
	Violations      []string           `json:"violations"`
}

// Run executes the command under an active chaos proxy and evaluates resilience SLOs.
func Run(opts Options) (*Result, error) {
	if opts.ProxyDef == "" {
		return nil, fmt.Errorf("proxy definition is required (--proxy 'name=listen->upstream')")
	}
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("command is required after '--'")
	}

	name, listen, upstream, err := parseProxyDef(opts.ProxyDef)
	if err != nil {
		return nil, err
	}

	p := proxy.NewProxy(name, listen, upstream)
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("failed starting gate proxy: %w", err)
	}
	defer p.Stop()

	if opts.Preset != "" {
		if err := presets.ApplyPreset(p, opts.Preset); err != nil {
			return nil, fmt.Errorf("failed applying preset %q: %w", opts.Preset, err)
		}
	}

	startTime := time.Now()

	// Execute user test command
	cmd := exec.Command(opts.Command[0], opts.Command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmdErr := cmd.Run()
	duration := time.Since(startTime)

	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	snap := p.Stats.Snapshot()

	result := &Result{
		Passed:          exitCode == 0,
		CommandExitCode: exitCode,
		DurationMs:      duration.Milliseconds(),
		Preset:          opts.Preset,
		Telemetry:       snap,
		Violations:      make([]string, 0),
	}

	if exitCode != 0 {
		result.Violations = append(result.Violations, fmt.Sprintf("command exited with non-zero code: %d", exitCode))
	}

	// Verify p99 latency SLO
	if opts.MaxP99Latency > 0 && snap.P99LatencyMs > float64(opts.MaxP99Latency.Milliseconds()) {
		result.Passed = false
		result.Violations = append(
			result.Violations,
			fmt.Sprintf("p99 latency (%.2fms) exceeded threshold (%dms)", snap.P99LatencyMs, opts.MaxP99Latency.Milliseconds()),
		)
	}

	// Verify error rate SLO
	if opts.MaxErrorRate > 0 && snap.TotalConnections > 0 {
		actualErrorRate := float64(snap.ErrorCount) / float64(snap.TotalConnections)
		if actualErrorRate > opts.MaxErrorRate {
			result.Passed = false
			result.Violations = append(
				result.Violations,
				fmt.Sprintf("error rate (%.2f%%) exceeded threshold (%.2f%%)", actualErrorRate*100, opts.MaxErrorRate*100),
			)
		}
	}

	// Emit report
	if err := emitReport(result, opts.ReportFormat, opts.ReportOutput); err != nil {
		return result, fmt.Errorf("failed writing report: %w", err)
	}

	return result, nil
}

func parseProxyDef(def string) (name, listen, upstream string, err error) {
	parts := strings.Split(def, "=")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid proxy format %q, expected 'name=listen->upstream'", def)
	}
	name = strings.TrimSpace(parts[0])

	arrowParts := strings.Split(parts[1], "->")
	if len(arrowParts) != 2 {
		return "", "", "", fmt.Errorf("invalid route format %q, expected 'listen->upstream'", parts[1])
	}
	listen = strings.TrimSpace(arrowParts[0])
	upstream = strings.TrimSpace(arrowParts[1])
	return name, listen, upstream, nil
}

func emitReport(res *Result, format, outputPath string) error {
	var writer io.Writer = os.Stdout
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer f.Close()
		writer = f
	}

	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	case "junit":
		return writeJUnitReport(writer, res)
	default: // Console output
		fmt.Fprintln(writer, "\n--- FaultCraft CI/CD Gate Summary ---")
		if res.Passed {
			fmt.Fprintln(writer, "[PASS] Resilience gate passed successfully.")
		} else {
			fmt.Fprintln(writer, "[FAIL] Resilience gate violated SLO requirements:")
			for _, v := range res.Violations {
				fmt.Fprintf(writer, "  * %s\n", v)
			}
		}
		fmt.Fprintf(writer, "Active Conns: %d | Total Conns: %d | Errors: %d | Injected Faults: %d\n",
			res.Telemetry.ActiveConnections, res.Telemetry.TotalConnections, res.Telemetry.ErrorCount, res.Telemetry.FaultCount)
		fmt.Fprintf(writer, "P50 Latency: %.2fms | P99 Latency: %.2fms\n", res.Telemetry.P50LatencyMs, res.Telemetry.P99LatencyMs)
		fmt.Fprintln(writer, "--------------------------------------")
		return nil
	}
}

// JUnit XML structures
type JUnitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Value   string `xml:",chardata"`
}

func writeJUnitReport(w io.Writer, res *Result) error {
	var failures int
	if !res.Passed {
		failures = len(res.Violations)
		if failures == 0 {
			failures = 1
		}
	}

	testCases := make([]JUnitTestCase, 0)

	// Test 1: Command execution
	cmdCase := JUnitTestCase{
		Name:      "CommandExecution",
		ClassName: "faultcraft.gate",
		Time:      float64(res.DurationMs) / 1000.0,
	}
	if res.CommandExitCode != 0 {
		cmdCase.Failure = &JUnitFailure{
			Message: fmt.Sprintf("command exited with code %d", res.CommandExitCode),
			Value:   fmt.Sprintf("Process exited with return code %d", res.CommandExitCode),
		}
	}
	testCases = append(testCases, cmdCase)

	// Test 2: Latency SLO
	latCase := JUnitTestCase{
		Name:      "LatencySLO",
		ClassName: "faultcraft.gate",
		Time:      res.Telemetry.P99LatencyMs / 1000.0,
	}
	for _, v := range res.Violations {
		if strings.Contains(v, "latency") {
			latCase.Failure = &JUnitFailure{
				Message: v,
				Value:   v,
			}
		}
	}
	testCases = append(testCases, latCase)

	// Test 3: Error Rate SLO
	errCase := JUnitTestCase{
		Name:      "ErrorRateSLO",
		ClassName: "faultcraft.gate",
		Time:      0,
	}
	for _, v := range res.Violations {
		if strings.Contains(v, "error rate") {
			errCase.Failure = &JUnitFailure{
				Message: v,
				Value:   v,
			}
		}
	}
	testCases = append(testCases, errCase)

	suites := JUnitTestSuites{
		Suites: []JUnitTestSuite{
			{
				Name:      "faultcraft-resilience-gate",
				Tests:     len(testCases),
				Failures:  failures,
				Time:      float64(res.DurationMs) / 1000.0,
				TestCases: testCases,
			},
		},
	}

	_, _ = io.WriteString(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(suites)
}
