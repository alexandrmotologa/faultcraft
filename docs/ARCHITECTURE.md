# Architecture and Internal Design

This document details the internal design of `faultcraft`, including the concurrency model, connection proxying loop, toxic pipeline execution, and memory management.

---

## 1. Core Concurrency and Lifecycle Model

`faultcraft` uses standard Go goroutines and non-blocking I/O primitives. The engine runs without external runtime dependencies or kernel drivers.

Each configured proxy runs an independent `net.Listener`. When a client connects:
1. An ingress goroutine accepts the raw TCP connection.
2. The proxy verifies whether the instance is enabled and limits the active socket pool if configured.
3. The forwarder initiates a standard TCP connection to the upstream target.
4. Two bidirectional streaming goroutines are spawned:
   - **Upstream Pipe**: Reads bytes from the client and writes to the target service.
   - **Downstream Pipe**: Reads bytes from the target service and writes back to the client.
5. Both pipes pass through directional chains of `Toxic` processors.

---

## 2. The Toxic Pipeline Engine

A `Toxic` is an interface with lifecycle methods executed on every read and write cycle:

```go
type Toxic interface {
    Name() string
    Type() string
    Stream() StreamDirection // Upstream or Downstream
    Toxicity() float64       // Probability (0.0 to 1.0)
    Pipe(ctx context.Context, link *Link) error
    Close() error
}
```

When data traverses a `Link`, the buffer passes sequentially through each active toxic in the chain:
- **Toxicity Check**: If `toxicity < 1.0`, a cryptographically pseudo-random float determines whether this specific connection experiences the fault.
- **Latency Toxic**: Buffers packets in a channel or sleeps using a Box-Muller Gaussian normal distribution with configurable mean and jitter.
- **Bandwidth Toxic**: Implements a token-bucket algorithm with microsecond granularity. If the byte count exceeds the bucket capacity, the write pauses until new tokens replenish.
- **Slicer Toxic**: Splits large byte slices into smaller chunks (e.g. 64 to 256 bytes) and injects microsecond pauses between TCP packets to test fragmented framing.
- **HTTP Error Toxic**: Inspects response headers on downstream links. If HTTP/1.1 framing is detected, it rewrites the status line to 500, 502, 503, or 504 and outputs a synthetic error payload.
- **Reset Peer Toxic**: Terminates the socket using `TCPConn.SetLinger(0)` followed by `Close()`, forcing the operating system to send a TCP RST packet instead of a graceful FIN exchange.
- **Hang Toxic**: Suspends reading and writing without closing the socket, simulating an unresponsive server thread.

---

## 3. Zero-Allocation Buffer Pooling

To prevent garbage collection pauses under high throughput, `faultcraft` uses `sync.Pool` for 32 KB I/O transfer buffers. Buffers are returned to the pool immediately upon completion of the pipe or connection teardown.

---

## 4. Telemetry and Server-Sent Events

Metrics are stored atomically in a concurrent registry:
- Active open socket count.
- Cumulative bytes transferred upstream and downstream.
- Total injected latency duration.
- Cumulative fault execution count.

A background broadcaster pushes snapshots every 500 milliseconds across active SSE client connections (`/api/v1/events`). This allows the React 19 cockpit to display live throughput charts without repetitive polling requests.
