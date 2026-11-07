# faultcraft

Self-hosted network chaos proxy and fault injection engine for TCP and HTTP microservices with an embedded React 19 matrix.

---

## Overview

Distributed services degrade through slow failures more often than outright crashes. Database lock spikes, third-party payment gateway timeouts, transient packet loss, and throttled network links lead to worker pool exhaustion and cascading failures.

`faultcraft` is a single-binary network fault injection proxy written in Go with an embedded React 19 interface. It intercepts TCP and HTTP/1.1 traffic between an application and upstream dependencies such as PostgreSQL, MySQL, Redis, RabbitMQ, or REST APIs.

Engineers can attach bidirectional fault pipelines ("toxics"), including Gaussian jitter latency, token-bucket bandwidth throttling, TCP socket freezes, connection resets, byte slicing, and probabilistic HTTP error status code rewrites. The system exposes full Toxiproxy v2 REST API compatibility, allowing existing test suites written for Toxiproxy client SDKs to work without code changes.

---

## Key Capabilities

* **High-Throughput TCP Proxy**: Asynchronous goroutine-based connection forwarding with zero external runtime dependencies.
* **Toxiproxy v2 Compatibility**: Drop-in compatible REST API matching Toxiproxy v2 routes and JSON schemas. Official Toxiproxy clients in Go, Python, Ruby, and Node.js work directly.
* **Directional Toxic Pipeline**: Apply independent fault chains upstream (client to proxy) or downstream (upstream target to client).
* **Supported Fault Injections**:
  * `latency`: Gaussian normal distribution delay with mean, jitter, and correlation factors.
  * `bandwidth`: Token-bucket rate limiter for bandwidth throttling down to specific KB/s rates.
  * `timeout`: Closes sockets after an idle or elapsed duration.
  * `slicer`: Breaks TCP streams into configurable chunks with inter-packet delays.
  * `hang`: Completes the TCP handshake but stops transmitting data.
  * `reset_peer`: Aborts connections using TCP RST packets.
  * `limit_data`: Enforces a maximum byte quota per socket before closing.
  * `http_error`: Intercepts HTTP/1.1 responses and probabilistically rewrites them to 500, 502, 503, or 504.
  * `corrupt`: Probabilistic payload byte alteration to test parser resilience.
* **Embedded React 19 Matrix**: Interactive web cockpit served directly from the Go binary via `embed.FS` on the management port.
* **Real-Time Telemetry**: Server-Sent Events (SSE) push stream for live throughput sparklines, active connection counts, and fault trigger counters.
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
  --report-format junit \
  --output test-results/resilience.xml \
  -- pytest tests/integration
```

If the test suite fails or error rates exceed the defined threshold, `faultcraft gate` exits with code 1.

---

## Documentation

* [Architecture & Networking](docs/ARCHITECTURE.md)
* [Toxiproxy Client Compatibility](docs/TOXIPROXY_COMPATIBILITY.md)
* [CI/CD Gating Guide](docs/CI_CD_GATING.md)
* [REST API Reference](docs/API_REFERENCE.md)

---

## License

MIT License. Copyright (c) 2026 Alexandr Motologa.
