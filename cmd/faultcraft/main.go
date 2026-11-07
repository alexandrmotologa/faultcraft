package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/api"
	"github.com/alexandrmotologa/faultcraft/internal/config"
	"github.com/alexandrmotologa/faultcraft/internal/gate"
	"github.com/alexandrmotologa/faultcraft/internal/presets"
	"github.com/alexandrmotologa/faultcraft/internal/proxy"
	"github.com/alexandrmotologa/faultcraft/web"
)

var (
	Version   = "0.1.0"
	GitCommit = "dev"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "start":
		runStart(os.Args[2:])
	case "gate":
		runGate(os.Args[2:])
	case "apply":
		runApply(os.Args[2:])
	case "reset":
		runReset(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("faultcraft %s (%s)\n", Version, GitCommit)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`faultcraft: Self-Hosted Network Chaos Proxy & Latency Fault Injection Engine

Usage:
  faultcraft <command> [flags]

Commands:
  start     Start the proxy daemon and embedded React 19 chaos matrix
  gate      Execute tests under active chaos and evaluate CI/CD resilience SLOs
  apply     Apply a chaos preset to a running daemon
  reset     Reset all toxics on a running daemon
  version   Display version and build information

Run 'faultcraft <command> --help' for details on each subcommand.`)
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	apiAddr := fs.String("api-addr", ":8474", "HTTP API and Web Dashboard address")
	configFile := fs.String("config", "", "Path to YAML/JSON configuration file")
	_ = fs.Parse(args)

	mgr := proxy.NewManager()
	defer mgr.Clear()

	// Load configuration file if specified
	if *configFile != "" {
		cfg, err := config.Load(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed loading config file: %v\n", err)
			os.Exit(1)
		}
		if cfg.APIAddr != "" && *apiAddr == ":8474" {
			*apiAddr = cfg.APIAddr
		}
		for _, pc := range cfg.Proxies {
			p := proxy.NewProxy(pc.Name, pc.Listen, pc.Upstream)
			if pc.Enabled != nil {
				p.Enabled = *pc.Enabled
			}
			if err := mgr.Add(p); err != nil {
				fmt.Fprintf(os.Stderr, "Failed adding proxy %s: %v\n", pc.Name, err)
			} else {
				fmt.Printf("[Proxy Registered] %s: %s -> %s\n", p.Name, p.Listen, p.Upstream)
				if pc.Preset != "" {
					_ = presets.ApplyPreset(p, pc.Preset)
					fmt.Printf("  └ Applied Preset: %s\n", pc.Preset)
				}
			}
		}
	}

	uiFileSystem := web.GetFS()
	srv := api.NewServer(mgr, uiFileSystem, Version)
	defer srv.Close()

	httpServer := &http.Server{
		Addr:    *apiAddr,
		Handler: srv.Router(),
	}

	go func() {
		fmt.Printf("\n⚡ FaultCraft Daemon v%s listening on http://localhost%s\n", Version, *apiAddr)
		fmt.Printf("   Toxiproxy v2 API: http://localhost%s/proxies\n", *apiAddr)
		fmt.Printf("   Interactive Cockpit: http://localhost%s/\n\n", *apiAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down FaultCraft daemon...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(ctx)
	mgr.Clear()
	fmt.Println("Clean shutdown complete.")
}

func runGate(args []string) {
	fs := flag.NewFlagSet("gate", flag.ExitOnError)
	proxyDef := fs.String("proxy", "", "Proxy route formatted as 'name=listen->upstream'")
	preset := fs.String("preset", "flaky_db", "Chaos preset ID to apply")
	maxP99 := fs.Duration("max-p99-latency", 1000*time.Millisecond, "Max permissible p99 latency SLO")
	maxErrorRate := fs.Float64("max-error-rate", 0.10, "Max permissible error rate (0.0 to 1.0)")
	reportFormat := fs.String("report-format", "console", "Report format ('console', 'json', 'junit')")
	reportOutput := fs.String("output", "", "Path to write report (default: stdout)")

	// Parse flags until '--'
	var cmdArgs []string
	for i, arg := range args {
		if arg == "--" {
			_ = fs.Parse(args[:i])
			cmdArgs = args[i+1:]
			break
		}
	}

	if len(cmdArgs) == 0 {
		_ = fs.Parse(args)
		fmt.Fprintln(os.Stderr, "Error: command to execute is required after '--'")
		fmt.Fprintln(os.Stderr, "Example: faultcraft gate --proxy 'pg=localhost:15432->localhost:5432' -- npm test")
		os.Exit(1)
	}

	opts := gate.Options{
		ProxyDef:      *proxyDef,
		Preset:        *preset,
		MaxP99Latency: *maxP99,
		MaxErrorRate:  *maxErrorRate,
		ReportFormat:  *reportFormat,
		ReportOutput:  *reportOutput,
		Command:       cmdArgs,
	}

	res, err := gate.Run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gate execution error: %v\n", err)
		os.Exit(1)
	}

	if !res.Passed {
		os.Exit(1)
	}
}

func runApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	apiURL := fs.String("api", "http://localhost:8474", "FaultCraft daemon API URL")
	proxyName := fs.String("proxy", "", "Proxy name to target")
	presetID := fs.String("preset", "", "Preset ID to apply")
	_ = fs.Parse(args)

	if *proxyName == "" || *presetID == "" {
		fmt.Fprintln(os.Stderr, "Error: --proxy and --preset flags are required")
		os.Exit(1)
	}

	payload, _ := json.Marshal(map[string]string{
		"proxy":  *proxyName,
		"preset": *presetID,
	})

	resp, err := http.Post(*apiURL+"/api/v1/presets/apply", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed connecting to daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Failed applying preset, status: %s\n", resp.Status)
		os.Exit(1)
	}

	fmt.Printf("✓ Applied preset %q to proxy %q\n", *presetID, *proxyName)
}

func runReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	apiURL := fs.String("api", "http://localhost:8474", "FaultCraft daemon API URL")
	_ = fs.Parse(args)

	resp, err := http.Post(*apiURL+"/reset", "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed connecting to daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Failed resetting proxies, status: %s\n", resp.Status)
		os.Exit(1)
	}

	fmt.Println("✓ All toxics cleared and all proxies re-enabled.")
}
