# faultcraft: Architecture and Engineering Master Plan

> Self-Hosted Network Chaos Proxy and Latency Fault Injection Engine (Toxiproxy Alternative)  
> Deterministic Microservices Resilience Testing in a Single Binary (Go + React 19)

---

## 1. Executive Summary and Problem Space

### The Problem
Distributed systems frequently degrade before they crash completely. High database lock contention, third-party payment gateway timeouts, transient Redis packet loss, and throttled network links lead to worker thread exhaustion, cascaded failure cascades, and circuit breaker misconfigurations.

Standard engineering practices lack approachable tools for local resilience verification:
1. **Shopify Toxiproxy**: The industry standard for simulated network faults, but limited by a lack of a real-time visual interface, manual JSON payload creation, and cumbersome observability.
2. **Kubernetes Chaos Frameworks (Chaos Mesh, LitmusChaos)**: Require cluster-level administrative rights, kernel-level eBPF or iptables manipulation, and complex CRD manifests, making them too heavy for local unit and integration tests on developer machines or lightweight CI/CD runners.

### The Solution: faultcraft
`faultcraft` is a lightweight, single-binary network fault injection engine written in Go with an embedded React 19 interactive chaos cockpit. It intercepts TCP and HTTP/1.1 traffic between client applications and upstream dependencies (PostgreSQL, MySQL, Redis, RabbitMQ, HTTP microservices).

Engineers can dynamically attach bidirectional fault pipelines ("toxics"), such as Gaussian jitter latency, token-bucket bandwidth throttling, TCP socket freezes, connection resets, byte slicing, and probabilistic HTTP error status code rewrites. Changes apply instantly through an embedded web matrix, a Toxiproxy v2-compatible REST API, or an automated CI/CD gating CLI.

---

## 2. System Architecture and Pipeline Flow

```
[ Application Client (App Code, Microservice, Test Suite) ]
                       |
                       |  Connects to Local Proxy Listener (e.g., :15432)
                       v
+------------------------------------------------------------------------+
|                        faultcraft Proxy Daemon                         |
|                                                                        |
|  +------------------------------------------------------------------+  |
|  | Ingress TCP Listener & Protocol Sniffer                          |  |
|  | - Buffers incoming data streams                                  |  |
|  | - Detects HTTP/1.1 framing vs. raw TCP stream                    |  |
|  +----------------------------------+-------------------------------+  |
|                                     |                                  |
|                                     v                                  |
|  +------------------------------------------------------------------+  |
|  | Bidirectional Fault Pipeline (Ordered Chain of Toxics)           |  |
|  |                                                                  |  |
|  |  [ Toxic 1: Latency ]       Delays packet transmission           |  |
|  |                             (Gaussian mean + jitter distribution) |  |
|  |  [ Toxic 2: Bandwidth ]     Token-bucket throughput throttle     |  |
|  |                             (Configurable KB/s rate limit)       |  |
|  |  [ Toxic 3: Slicer ]        Splits stream into micro-chunks      |  |
|  |                             with inter-packet delays             |  |
|  |  [ Toxic 4: Error Inject ]  Rewrites HTTP status to 500/502/503  |  |
|  |  [ Toxic 5: Hang / Freeze ] Stalls socket without FIN packets    |  |
|  |  [ Toxic 6: Reset Peer ]    Abruptly terminates via TCP RST      |  |
|  |  [ Toxic 7: Data Limit ]    Closes connection after N bytes      |  |
|  |  [ Toxic 8: Corrupt ]       Flips payload bits / tampers bytes   |  |
|  +----------------------------------+-------------------------------+  |
|                                     |                                  |
|                                     v                                  |
|  +------------------------------------------------------------------+  |
|  | Egress Forwarder to Target Service (PostgreSQL, Redis, HTTP API) |  |
|  +------------------------------------------------------------------+  |
|                                                                        |
|  +------------------------------------------------------------------+  |
|  | Embedded React 19 Interactive Chaos Cockpit                      |  |
|  | - Real-time active connection tracking and byte throughput gauges|  |
|  | - 1-Click presets: Flaky Database, Mobile 3G, Gateway Outage     |  |
|  | - Live Server-Sent Events (SSE) telemetry stream                 |  |
|  | - CLI / curl script exporter for CI/CD reproduction              |  |
|  +------------------------------------------------------------------+  |
|                                                                        |
|  +------------------------------------------------------------------+  |
|  | Toxiproxy v2 REST API Engine + Automated CI/CD Gating Runner     |  |
|  +------------------------------------------------------------------+  |
+------------------------------------------------------------------------+
```

---

## 3. Technology Stack

| Layer | Component | Specification |
| :--- | :--- | :--- |
| **Backend Runtime** | Go 1.23+ | Non-blocking goroutine networking, standard library `net` package, zero CGo dependencies |
| **Network Core** | Bidirectional TCP Streamer | Full duplex streaming with thread-safe toxic chain execution and atomic stats |
| **Protocol Support** | TCP + HTTP/1.1 | Raw binary TCP forwarding with specialized HTTP/1.1 response inspection and modification |
| **API Compatibility** | Toxiproxy v2 REST API | 100% route and schema compatibility with official Toxiproxy client SDKs |
| **Telemetry** | Server-Sent Events (SSE) | Real-time push stream of metrics, active connections, and fault logs |
| **Frontend Framework** | React 19 + TypeScript + Vite | Embedded inside Go binary using `//go:embed` with zero external dependencies |
| **Styling & Icons** | Tailwind CSS + Lucide Icons | Dark engineering dashboard layout with responsive mobile and desktop views |
| **CLI & Testing** | Go CLI + Test Automation | Integrated `gate` command for CI/CD test harness validation |

---

## 4. Repository Structure

```
B:\workgit\faultcraft\
├── cmd\
│   └── faultcraft\
│       └── main.go                 # Proxy daemon entrypoint and CLI subcommands
├── internal\
│   ├── api\
│   │   ├── router.go               # HTTP router and middleware
│   │   ├── proxies.go              # Toxiproxy v2 proxy endpoints
│   │   ├── toxics.go               # Toxiproxy v2 toxic endpoints
│   │   ├── presets.go              # Chaos preset management endpoints
│   │   ├── scenarios.go            # Timed chaos sequence runner
│   │   └── sse.go                  # Server-Sent Events metrics broadcaster
│   ├── config\
│   │   └── config.go               # Configuration structures, defaults, and flag parsing
│   ├── gate\
│   │   └── runner.go               # CI/CD gate executor with SLO assertions and JUnit output
│   ├── presets\
│   │   └── builtin.go              # Built-in chaos profiles (Flaky DB, 3G, Outage, Packet Shredder)
│   ├── proxy\
│   │   ├── manager.go              # Multi-proxy registry and lifecycle coordinator
│   │   ├── proxy.go                # Single TCP proxy listener and connection dispatcher
│   │   ├── connection.go           # Full-duplex connection pipe with toxic chain integration
│   │   └── stats.go                # Atomic metrics counters (bytes, latency, active sockets)
│   └── toxics\
│       ├── toxic.go                # Toxic interface and directional pipeline
│       ├── latency.go              # Gaussian distribution delay injector
│       ├── bandwidth.go            # Token-bucket rate limiter
│       ├── timeout.go              # Connection cutoff after duration
│       ├── slicer.go               # Packet chunking with microsecond delay
│       ├── hang.go                 # Half-open socket silence simulator
│       ├── reset.go                # TCP RST socket abort
│       ├── limit_data.go           # Quota-based socket termination
│       ├── http_error.go           # HTTP response rewrite (500/502/503/504)
│       └── corrupt.go              # Bit-flip and byte tampering toxic
├── web\
│   ├── index.html                  # Single page application entrypoint
│   ├── package.json                # React 19 dependencies and scripts
│   ├── tsconfig.json               # TypeScript compiler config
│   ├── vite.config.ts              # Vite bundle builder with embedded proxy support
│   └── src\
│       ├── App.tsx                 # Root layout and state orchestrator
│       ├── main.tsx                # React DOM mount point
│       ├── types.ts                # TypeScript schemas matching Go API
│       ├── components\
│       │   ├── Header.tsx          # Top navigation and status bar
│       │   ├── ProxyList.tsx       # Active proxies card overview
│       │   ├── ProxyCard.tsx       # Individual proxy controls and live metrics
│       │   ├── ToxicList.tsx       # Configured toxics per proxy
│       │   ├── ToxicModal.tsx      # Modal for configuring and attaching toxics
│       │   ├── PresetBar.tsx       # 1-Click quick chaos presets
│       │   ├── ScenarioPlayer.tsx  # Timed multi-step chaos recipe runner
│       │   ├── TrafficChart.tsx    # Live throughput visualizer
│       │   └── CliExportModal.tsx  # Reproducible curl and CLI command generator
│       └── services\
│           ├── api.ts              # REST client for faultcraft backend
│           └── sse.ts              # SSE subscription hook
├── docs\
│   ├── ARCHITECTURE.md             # Detailed network pipe design and lifecycle
│   ├── TOXIPROXY_COMPATIBILITY.md  # Compatibility guide with Toxiproxy SDKs
│   ├── CI_CD_GATING.md             # Guide for integrating with GitHub Actions and test suites
│   └── API_REFERENCE.md            # Complete REST and SSE endpoint documentation
├── scripts\
│   └── demo_upstream.go            # Mock PostgreSQL, Redis, and HTTP echo server for testing
├── Makefile                        # Build, test, and release recipes
├── Dockerfile                      # Minimal multi-stage build image
├── README.md                       # Product documentation
└── MASTER_PLAN.md                  # This document
```

---

## 5. Supported Toxics and Specifications

| Toxic Name | Direction | Parameters | Description |
| :--- | :--- | :--- | :--- |
| **`latency`** | Up / Down | `latency` (int ms), `jitter` (int ms), `correlation` (float) | Injects normal distribution delays. Simulates inter-datacenter latency and database locks. |
| **`bandwidth`** | Up / Down | `rate` (int KB/s) | Limits maximum transfer rate using a token-bucket algorithm. Simulates bandwidth-constrained mobile or edge links. |
| **`timeout`** | Up / Down | `timeout` (int ms) | Closes the connection abruptly after specified idle duration. Tests client read timeouts. |
| **`slicer`** | Up / Down | `average_size` (int bytes), `size_variation` (int bytes), `delay` (int us) | Slices TCP stream into small chunks with brief microsecond pauses. Tests buffer boundary parsing. |
| **`hang`** | Up / Down | None | Completes the TCP handshake but never forwards data. Tests client hung connection and dead worker handling. |
| **`reset_peer`** | Up / Down | `timeout` (int ms) | Sends an ungraceful TCP RST abort (`SO_LINGER = 0`). Simulates sudden process crash or network partition. |
| **`limit_data`** | Up / Down | `bytes` (int bytes) | Closes the connection immediately once transferred bytes reach the quota. |
| **`http_error`** | Downstream | `status` (int, default 503), `rate` (float 0.0-1.0), `body` (string) | Intercepts HTTP/1.1 response headers and probabilistically rewrites them to synthetic error responses. |
| **`corrupt`** | Up / Down | `rate` (float 0.0-1.0), `byte_index` (int) | Alters random bytes in the payload to test protocol parser resilience. |

---

## 6. Built-in Chaos Presets

1. **Flaky Database (`flaky_db`)**:
   - Ingress Latency: 180ms +/- 40ms jitter
   - Reset Peer: 3% probability after 2000ms
   - Simulates overloaded PostgreSQL or MySQL connection pool under lock contention.

2. **Degraded 3G Cellular (`cellular_3g`)**:
   - Upstream and Downstream Bandwidth: 256 KB/s
   - Downstream Latency: 350ms +/- 80ms jitter
   - Slicer: 256 byte chunks with 2000us delay

3. **Payment Gateway Outage (`payment_outage`)**:
   - Downstream HTTP Error: 504 Gateway Timeout on 100% of requests
   - Simulates downstream third-party payment vendor outage.

4. **Zombie Socket (`zombie_worker`)**:
   - Upstream Hang: Socket connects and never replies.
   - Tests if client worker pool enforces strict read deadlines.

5. **Packet Shredder (`packet_shredder`)**:
   - Slicer: 64 byte chunks
   - Latency: 100ms jitter
   - Corrupt: 1% payload corruption

---

## 7. Implementation Roadmap

### Phase 1: High-Performance TCP Streaming Core & Statistics
- [ ] Initialize Go module `github.com/alexandrmotologa/faultcraft`.
- [ ] Implement `internal/proxy/proxy.go` with high-concurrency TCP listener, bidirectional piping, and graceful teardown.
- [ ] Implement `internal/proxy/connection.go` with zero-allocation buffer pools (`sync.Pool`).
- [ ] Implement `internal/proxy/stats.go` tracking atomic byte counters, active sockets, and latency metrics.
- [ ] Build standalone mock echo/HTTP server in `scripts/demo_upstream.go` for rapid integration tests.

### Phase 2: Modular Toxic Engine & Network Fault Injections
- [ ] Implement `Toxic` interface and bidirectional pipeline runner in `internal/toxics/toxic.go`.
- [ ] Implement `latency.go` with Box-Muller Gaussian normal distribution and jitter correlation.
- [ ] Implement `bandwidth.go` using token-bucket rate limiting with microsecond granularity.
- [ ] Implement `slicer.go` for TCP buffer fragmentation.
- [ ] Implement `timeout.go`, `hang.go`, `reset.go` (TCP RST flag), and `limit_data.go`.
- [ ] Implement `http_error.go` for HTTP/1.1 response status code interception and rewriting.
- [ ] Implement `corrupt.go` for probabilistic payload alteration.
- [ ] Write comprehensive unit and integration tests verifying each toxic under real network load.

### Phase 3: Toxiproxy v2 API, Presets & CI/CD Gating
- [ ] Implement Toxiproxy v2-compatible REST router in `internal/api/router.go`.
- [ ] Implement `/proxies` and `/proxies/{name}/toxics` CRUD endpoints matching standard JSON schemas.
- [ ] Implement SSE telemetry stream at `/api/v1/events` for real-time frontend updates.
- [ ] Implement preset management and scenario runner (`/api/v1/presets`, `/api/v1/scenarios`).
- [ ] Implement `faultcraft gate` CLI command for CI/CD test runners with latency/error SLO checks.

### Phase 4: Embedded React 19 Interactive Chaos Cockpit
- [ ] Scaffold Vite + React 19 + TypeScript app in `web/`.
- [ ] Configure Tailwind CSS and Lucide icons with dark engineering visual identity.
- [ ] Build `ProxyList`, `ProxyCard`, and `ToxicList` components with live toggle controls.
- [ ] Build `ToxicModal` for adjusting delay, jitter, rate limits, and error rates.
- [ ] Build `TrafficChart` visualizing live throughput and active connection counts.
- [ ] Build `PresetBar` for one-click chaos activation.
- [ ] Build `CliExportModal` providing instant `curl` and `faultcraft` CLI commands.
- [ ] Integrate React build artifacts into Go executable with `//go:embed`.

### Phase 5: Verification, Documentation & Release
- [ ] Write end-to-end integration tests in Go exercising the full stack (proxy, toxics, API, gating).
- [ ] Verify zero socket leaks under sustained concurrent connections.
- [ ] Write comprehensive documentation in `docs/` and humanized `README.md`.
- [ ] Build cross-platform binaries (Linux, macOS, Windows) and Dockerfile.
