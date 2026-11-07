# Toxiproxy Compatibility Guide

`faultcraft` implements the Toxiproxy v2 HTTP REST API specification. Existing integration test suites that rely on official or community Toxiproxy client SDKs can point directly to `faultcraft` without code modifications.

---

## Supported Client SDKs

The following clients have been verified against `faultcraft`:

* **Go**: `github.com/Shopify/toxiproxy/v2/client`
* **Python**: `toxiproxy-python`
* **Ruby**: `toxiproxy-ruby`
* **Node.js**: `toxiproxy-node-client` / `toxiproxy`
* **Rust**: `toxiproxy-rs`

---

## Wire-Level Endpoints

| Method | Path | Description | Toxiproxy v2 Schema Match |
| :--- | :--- | :--- | :--- |
| `GET` | `/version` | Returns daemon version string | Yes |
| `GET` | `/proxies` | Map of all registered proxies | Yes |
| `POST` | `/proxies` | Create a new proxy | Yes |
| `POST` | `/populate` | Batch creation of multiple proxies | Yes |
| `GET` | `/proxies/{name}` | Get details of a proxy | Yes |
| `POST` | `/proxies/{name}` | Update listen/upstream or enable/disable | Yes |
| `DELETE` | `/proxies/{name}` | Delete a proxy | Yes |
| `GET` | `/proxies/{name}/toxics` | List all toxics attached to a proxy | Yes |
| `POST` | `/proxies/{name}/toxics` | Attach a new toxic | Yes |
| `GET` | `/proxies/{name}/toxics/{toxic_name}` | Get toxic configuration | Yes |
| `POST` | `/proxies/{name}/toxics/{toxic_name}` | Update toxic attributes | Yes |
| `DELETE` | `/proxies/{name}/toxics/{toxic_name}` | Remove a toxic | Yes |
| `POST` | `/reset` | Clear all toxics and re-enable all proxies | Yes |

---

## Example: Using Python Toxiproxy Client

```python
from toxiproxy import Toxiproxy

# Connect directly to faultcraft daemon on port 8474
client = Toxiproxy(host="localhost", port=8474)

# Create a proxy for Postgres
proxy = client.create(name="postgres", listen="localhost:15432", upstream="localhost:5432")

# Add downstream latency toxic
proxy.add_toxic(
    name="latency_down",
    type="latency",
    stream="downstream",
    toxicity=1.0,
    attributes={"latency": 200, "jitter": 30}
)

# Run integration tests against localhost:15432...

# Cleanup
client.reset()
```

---

## Example: Using Go Toxiproxy Client

```go
package main

import (
    "testing"
    toxiproxy "github.com/Shopify/toxiproxy/v2/client"
)

func TestDatabaseResilience(t *testing.T) {
    client := toxiproxy.NewClient("localhost:8474")

    proxy, err := client.CreateProxy("redis_chaos", "localhost:16379", "localhost:6379")
    if err != nil {
        t.Fatalf("failed to create proxy: %v", err)
    }
    defer proxy.Delete()

    // Inject bandwidth throttling
    _, err = proxy.AddToxic("slow_pipe", "bandwidth", "upstream", 1.0, toxiproxy.Attributes{
        "rate": 128, // 128 KB/s
    })
    if err != nil {
        t.Fatalf("failed to add toxic: %v", err)
    }

    // Assert your client handles slow responses or fails over cleanly
}
```
