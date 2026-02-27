# k6 Load Tests — lab_gear API

This directory contains [k6](https://k6.io/) load testing scripts for the `lab_gear` REST API.

## Directory Layout

```
k6/
├── lib/
│   ├── auth.js           # Authentication helpers and base URL configuration
│   └── helpers.js        # Test data generators and utility functions
└── scripts/
    ├── smoke.js          # Smoke test — verifies all endpoints work (1 VU, 1 iteration)
    ├── load.js           # Load test — normal sustained traffic (10 VUs, ~7 minutes)
    ├── stress.js         # Stress test — ramps to 100 VUs to find breaking point
    ├── soak.js           # Soak test — sustained moderate load for 40 minutes
    └── machines-crud.js  # CRUD lifecycle test — end-to-end data consistency (15 VUs, ~5 minutes)
```

## Prerequisites

Install k6 using one of these methods:

```bash
# macOS
brew install k6

# Linux (Debian/Ubuntu)
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
  --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6

# Docker (no install required)
docker run --rm -i grafana/k6 run - < k6/scripts/smoke.js
```

See the [official k6 installation docs](https://grafana.com/docs/k6/latest/set-up/install-k6/) for other platforms.

## Environment Variables

| Variable    | Required | Default                  | Description                          |
|-------------|----------|--------------------------|--------------------------------------|
| `API_TOKEN` | Yes      | —                        | Bearer token for API authentication  |
| `BASE_URL`  | No       | `http://localhost:8080`  | Base URL of the lab_gear service     |

## Running the Service Locally

Before running tests, start the service:

```bash
# From the repo root
export API_TOKEN=my-secret-token
make run

# Or with Docker
docker build -t lab_gear .
docker run -p 8080:8080 -e API_TOKEN=my-secret-token lab_gear
```

## Running the Tests

All scripts are run from the **repo root** directory so that the `../lib/` imports resolve correctly.

### Smoke Test

Run this first to confirm the service is reachable and all endpoints respond correctly.

```bash
k6 run -e API_TOKEN=my-secret-token k6/scripts/smoke.js

# Against a remote server
k6 run -e BASE_URL=https://lab.example.com -e API_TOKEN=my-secret-token k6/scripts/smoke.js
```

Expected: 1 VU, 1 iteration, all checks pass, completes in seconds.

---

### Load Test

Simulates normal steady-state traffic with a read-heavy mix (70% list, 20% get, 10% writes).

```bash
k6 run -e API_TOKEN=my-secret-token k6/scripts/load.js
```

Duration: ~7 minutes (1m ramp-up + 5m steady + 1m ramp-down)
VUs: up to 10

Thresholds:
- `p(99) < 500ms`
- `p(95) < 250ms`
- Error rate `< 1%`

---

### Stress Test

Ramps load up to 100 VUs to find the service's breaking point and verify recovery.

```bash
k6 run -e API_TOKEN=my-secret-token k6/scripts/stress.js
```

Duration: ~14 minutes
VUs: 20 → 50 → 100 → 0

Thresholds (relaxed for stress):
- `p(95) < 2s`
- Error rate `< 5%` (allows some degradation)

---

### Soak Test

Runs moderate load for 30 minutes to detect memory leaks and gradual degradation.

```bash
k6 run -e API_TOKEN=my-secret-token k6/scripts/soak.js
```

Duration: ~40 minutes (5m ramp-up + 30m sustained + 5m ramp-down)
VUs: 10

Thresholds:
- `p(99) < 1s` for the entire run
- Error rate `< 1%`

---

### CRUD Lifecycle Test

Each VU runs the full Create → Read → Update → Read → Delete → Tombstone-check workflow,
verifying data consistency under concurrent load.

```bash
k6 run -e API_TOKEN=my-secret-token k6/scripts/machines-crud.js
```

Duration: ~5 minutes
VUs: up to 15

Thresholds:
- Check pass rate `> 99%`
- Error rate `< 1%`
- `p(95) < 1s`

---

## Test Output

k6 prints a summary table at the end of each run. Key metrics to review:

| Metric                | What it tells you                                 |
|-----------------------|---------------------------------------------------|
| `http_req_duration`   | Response time percentiles (p50, p90, p95, p99)    |
| `http_req_failed`     | Fraction of requests that resulted in network/5xx errors |
| `checks`              | Pass rate for all explicit check assertions       |
| `http_reqs`           | Total request throughput (req/s)                  |
| `vus_max`             | Peak concurrent virtual users                     |

A non-zero exit code from k6 means one or more thresholds were breached.

## CI Integration

To add a smoke test to CI (e.g., GitHub Actions), start the service in the background and run:

```yaml
- name: Start lab_gear
  run: |
    export API_TOKEN=ci-test-token
    ./bin/lab_gear &
    sleep 2

- name: Run k6 smoke test
  run: k6 run -e API_TOKEN=ci-test-token k6/scripts/smoke.js
```

## Valid Machine Kinds

The API enforces these values for the `kind` field:

- `proxmox`
- `nas`
- `sbc`
- `bare_metal`
- `workstation`
- `laptop`
