<p align="center">
  <img src="docs/images/logo.png?raw=true" alt="FaultCraft Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">faultcraft</h1>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.23+-00ADD8.svg?style=flat-square&logo=go" alt="Go 1.23+" />
  <img src="https://img.shields.io/badge/react-19-61DAFB.svg?style=flat-square&logo=react" alt="React 19" />
  <img src="https://img.shields.io/badge/toxiproxy-v2_compatible-059669.svg?style=flat-square" alt="Toxiproxy v2 Compatible" />
  <img src="https://img.shields.io/badge/chaos-10_toxics-E11D48.svg?style=flat-square" alt="10 Toxics" />
  <img src="https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square" alt="MIT License" />
</p>

<p align="center">
  Self-hosted network chaos proxy and latency fault injection engine for TCP and HTTP microservices with Toxiproxy v2 API wire compatibility and embedded React 19 matrix.
</p>

<p align="center">
  <img src="docs/images/faultcraft_demo.gif?raw=true" alt="FaultCraft Live Interactive Demo" width="100%" />
</p>

---

## Overview

Distributed systems degrade through slow failures more often than clean crashes. Database lock contention, upstream payment gateway latency, transient TCP packet drops, and throttled network links exhaust connection pools and trigger cascading outages across dependent services.

`faultcraft` is a single-binary network fault injection engine written in Go with an embedded React 19 web cockpit. It acts as a transparent TCP and HTTP/1.1 proxy sitting between your application and upstream dependencies such as PostgreSQL, MySQL, Redis, RabbitMQ, or external REST APIs.

Engineers can attach directional fault pipelines ("toxics"), including Gaussian jitter latency, token-bucket bandwidth throttling, TCP socket freezes, connection resets, byte slicing, and probabilistic HTTP error status code rewrites. The system exposes full Toxiproxy v2 REST API compatibility, allowing existing test suites written for Toxiproxy client SDKs to work without code modifications.

---

## Interactive Cockpit & Dissector

### Desktop Chaos Matrix

The management cockpit runs on port 8474 directly from the Go binary via embedded assets. It provides real-time Server-Sent Events (SSE) telemetry, live bandwidth sparklines, 1-click chaos presets, auto-heal expiration badges, and upstream target reachability probing.

<p align="center">
  <img src="docs/images/faultcraft_dashboard.png?raw=true" alt="FaultCraft Desktop Dashboard" width="100%" />
</p>

### L7 Protocol Dissector & Live Inspector

The built-in packet dissector inspects incoming and outgoing TCP payloads, decoding HTTP/1.1 methods and response codes, Redis RESP commands (`GET`, `SET`, `PING`), and PostgreSQL Simple Query statements in real time.

<p align="center">
  <img src="docs/images/faultcraft_inspector.png?raw=true" alt="FaultCraft L7 Traffic Inspector" width="100%" />
</p>

### Responsive Mobile Interface

The entire management matrix is responsive across mobile, tablet, and desktop viewports, enabling remote drill execution and monitoring from mobile devices.

<p align="center">
  <img src="docs/images/faultcraft_mobile.png?raw=true" alt="FaultCraft Mobile Viewport" width="340" />
</p>

---

## Key Capabilities

* **High-Throughput TCP Proxy**: Non-blocking goroutine connection forwarding with buffer pooling (`sync.Pool`) and zero external runtime dependencies.
* **Toxiproxy v2 Drop-in Compatibility**: Wire-compatible REST API matching Toxiproxy v2 routes and JSON payload schemas. Official Toxiproxy clients in Go, Python, Ruby, and Node.js connect without alterations.
* **Directional Toxic Pipeline**: Apply independent fault chains upstream (client to proxy) or downstream (upstream target to client).
* **Supported Fault Injections (10 Toxics)**:
  * `latency`: Gaussian normal distribution delay with mean, jitter, and correlation factors.
  * `bandwidth`: Token-bucket rate limiter for bandwidth throttling down to specific KB/s rates.
  * `timeout`: Closes sockets after an idle or elapsed duration.
  * `slicer`: Breaks TCP streams into configurable chunks with inter-packet delays.
  * `hang`: Completes the TCP handshake but stops transmitting data.
  * `reset_peer`: Aborts connections using immediate or delayed TCP RST packets (`SetLinger(0)`).
  * `limit_data`: Enforces a maximum byte quota per socket before closing.
  * `http_error`: Intercepts HTTP/1.1 responses and probabilistically rewrites status codes (500, 502, 503, 504).
  * `circuit_breaker_probe`: Simulates an upstream service that trips client circuit breakers then enters cooldown and half-open probing.
  * `corrupt`: Probabilistic payload byte alteration to test parser resilience.
* **Auto-Heal Expiration Timers**: Optional TTL on any toxic that automatically removes the fault after specified seconds, preventing forgotten local chaos states from breaking development databases.
* **Upstream Target Health Probing**: Background prober testing physical connectivity and round-trip time (RTT) to target hosts every 3 seconds.
* **Incident Timeline Player**: Automated phased incident drill runner (`ScenarioPlayer`) executing multi-step failure scenarios with live step countdowns.
* **Config Export & Import**: JSON backup and restoration of all proxies and active toxics from the web UI or CLI.
* **CI/CD Quality Gate**: Command-line test harness runner (`faultcraft gate`) that executes test commands under active chaos profiles and enforces p99 latency and error rate thresholds.

---

## Installation

### Prebuilt Binaries

Download the appropriate binary for your operating system and architecture from GitHub Releases:

```bash
# Linux (amd64)
curl -L -o faultcraft https://github.com/alexandrmotologa/faultcraft/releases/latest/download/faultcraft-linux-amd64
chmod +x faultcraft
sudo mv faultcraft /usr/local/bin/

# macOS (Apple Silicon / arm64)
curl -L -o faultcraft https://github.com/alexandrmotologa/faultcraft/releases/latest/download/faultcraft-darwin-arm64
chmod +x faultcraft
sudo mv faultcraft /usr/local/bin/

# Windows (amd64)
# Download faultcraft-windows-amd64.exe from the Releases page.
```

### From Source

Ensure Go 1.23 or newer and Node.js 20 or newer are installed:

```bash
git clone https://github.com/alexandrmotologa/faultcraft.git
cd faultcraft
make build
./bin/faultcraft --help
```

---

## Quick Start

### 1. Start the Proxy Daemon

Start `faultcraft` listening for administrative requests on port 8474:

```bash
faultcraft start --api-addr ":8474"
```

Open `http://localhost:8474` in your browser to access the embedded React 19 matrix.

### 2. Create a Proxy

Route traffic from local port `15432` to a local PostgreSQL instance running on port `5432`:

```bash
curl -X POST http://localhost:8474/proxies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "postgres_chaos",
    "listen": "localhost:15432",
    "upstream": "localhost:5432",
    "enabled": true
  }'
```

Point your application database connection string to `postgres://user:pass@localhost:15432/mydb`.

### 3. Inject Latency

Add a downstream delay of 250 milliseconds with 50 milliseconds jitter:

```bash
curl -X POST http://localhost:8474/proxies/postgres_chaos/toxics \
  -H "Content-Type: application/json" \
  -d '{
    "name": "db_latency",
    "type": "latency",
    "stream": "downstream",
    "toxicity": 1.0,
    "attributes": {
      "latency": 250,
      "jitter": 50
    }
  }'
```

### 4. Remove Faults and Reset

Clear all active toxics on all proxies:

```bash
curl -X POST http://localhost:8474/reset
```

---

## CI/CD Quality Gating

Use `faultcraft gate` in your CI/CD pipeline to verify that your service handles faults gracefully:

```bash
faultcraft gate \
  --proxy "postgres_chaos=localhost:15432->localhost:5432" \
  --preset flaky_db \
  --max-p99-latency 500ms \
  --max-error-rate 0.05 \
  --report-format console \
  -- pytest tests/integration
```

Console summary output:

```text
--- FaultCraft CI/CD Gate Summary ---
[PASS] Resilience gate passed successfully.
Active Conns: 12 | Total Conns: 48 | Errors: 1 | Injected Faults: 14
P50 Latency: 112.40ms | P99 Latency: 238.10ms
--------------------------------------
```

If the test command fails or telemetry violates the p99 latency or error rate SLO thresholds, `faultcraft gate` exits with non-zero exit code 1 to block faulty code from reaching production.

---

## Documentation

* [Architecture & Networking](docs/ARCHITECTURE.md)
* [Toxiproxy Client Compatibility](docs/TOXIPROXY_COMPATIBILITY.md)
* [CI/CD Gating Guide](docs/CI_CD_GATING.md)
* [REST API Reference](docs/API_REFERENCE.md)

---

## License

MIT License. Copyright (c) 2026 Alexandr Motologa.
