# CI/CD Automated Gating Guide

`faultcraft` includes a dedicated `gate` subcommand designed to run inside continuous integration pipelines (GitHub Actions, GitLab CI, CircleCI). It executes test suites under controlled chaos conditions and fails the build if service-level objectives (SLOs) are violated.

---

## 1. Why Gate Resilience in CI?

Unit tests running on fast local loopback interfaces mask real-world network edge cases:
- Inadequate read and write socket timeouts.
- Thread exhaustion during slow upstream database responses.
- Missing circuit breaker and fallback handlers.

Running tests under active network faults proves that applications recover without crashing.

---

## 2. Using the CLI Gate Runner

```bash
faultcraft gate [flags] -- [command to execute]
```

### Supported Flags

| Flag | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| `--proxy` | string | Proxy definition formatted as `name=listen->upstream` | Required |
| `--preset` | string | Built-in chaos preset (`flaky_db`, `cellular_3g`, `payment_outage`) | `flaky_db` |
| `--max-p99-latency` | duration | Maximum permissible p99 latency before gating fails | `1000ms` |
| `--max-error-rate` | float | Maximum permissible ratio of connection errors (0.0 to 1.0) | `0.10` |
| `--report-format` | string | Output format (`junit`, `json`, `console`) | `console` |
| `--output` | string | File path for test report | stdout |

---

## 3. GitHub Actions Workflow Example

```yaml
name: Resilience Test Suite

on:
  push:
    branches: [ main ]
  pull_request:

jobs:
  chaos-gate:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: secret
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - name: Install FaultCraft
        run: |
          curl -sSL -o faultcraft https://github.com/alexandrmotologa/faultcraft/releases/latest/download/faultcraft-linux-amd64
          chmod +x faultcraft
          sudo mv faultcraft /usr/local/bin/

      - name: Run Integration Tests Under Fault Conditions
        run: |
          faultcraft gate \
            --proxy "pg_chaos=127.0.0.1:15432->127.0.0.1:5432" \
            --preset flaky_db \
            --max-p99-latency 600ms \
            --max-error-rate 0.05 \
            --report-format junit \
            --output test-results/chaos-gate.xml \
            -- npm test -- --runInBand

      - name: Upload Test Report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: chaos-gate-report
          path: test-results/
```
