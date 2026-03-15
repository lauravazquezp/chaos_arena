# Chaos Arena

A gamified chaos engineering platform. Inject failures into local Docker containers, watch a self-healing reconciliation loop respond in real time, and review post-mortem reports.

## Architecture

```
service-a → service-b → service-c
     ↑           ↑           ↑
     └───────────┴───────────┘
           control-plane
         (reconciler + API)
                ↑
            frontend
          (React + D3)
```

- **control-plane** — Go HTTP server. Manages arena state, dispatches attacks, runs a 500 ms reconciliation loop, streams events over WebSocket.
- **arena-service** — Minimal Node.js/Express target (three instances: `service-a`, `service-b`, `service-c`). Has `stress-ng`, `iproute2`, and `iptables` installed for fault injection.
- **frontend** — Vite + React + D3.js topology dashboard.

## Requirements

- Docker Desktop (macOS)
- `make`, `curl`, `jq` (for smoke test)

## Quick start

```bash
make up
```

- Dashboard: http://localhost:3000
- API: http://localhost:8080

## Make targets

| Target | Description |
|--------|-------------|
| `make up` | Build and start all services in detached mode |
| `make down` | Stop and remove containers |
| `make build` | Build images without starting |
| `make test` | Run Go unit tests (via Docker) |
| `make smoke` | Full end-to-end smoke test |
| `make logs` | Follow all container logs |
| `make clean` | Stop containers and remove volumes |
| `make grafana` | Start with Prometheus + Grafana overlay |

## API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/game/start` | Start a simulation → `{ "game_id": "..." }` |
| `POST` | `/game/stop` | End simulation, write post-mortem → report JSON |
| `GET` | `/game/state` | Current arena state |
| `POST` | `/attack` | `{ "service": "service-b", "attack": "kill_switch" }` |
| `GET` | `/reports` | List report filenames |
| `GET` | `/reports/{game_id}` | Fetch post-mortem JSON |
| `GET` | `/metrics` | Prometheus text format |
| `GET` | `/healthz` | Health check |
| `GET` | `/ws` | WebSocket — streams `Event` objects |

## Attacks

| Attack | Effect | Auto-heals |
|--------|--------|------------|
| `kill_switch` | Stops the container | Yes — restarts it |
| `black_hole` | `iptables -I INPUT -j DROP` | Yes — flushes rules |
| `latency_spike` | `tc netem delay 500ms` on eth0 | No (manual undo) |
| `resource_hog` | `stress-ng --cpu 4 --timeout 30s` | No (self-terminates) |

## Grafana overlay

### Start

```bash
make grafana
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 — credentials: `admin` / `admin`

> This is a local Docker container, not grafana.com. Use the credentials above regardless of any cloud account.

### Connect Prometheus as a data source

1. Open http://localhost:3001 and log in.
2. Go to **Connections → Data sources → Add new data source**.
3. Select **Prometheus**.
4. Set the URL to `http://prometheus:9090` (use the Docker service name, not localhost).
5. Click **Save & test** — you should see "Successfully queried the Prometheus API".

### Available metrics

All metrics are exposed at `http://localhost:8080/metrics` and scraped by Prometheus every 5 seconds.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chaos_arena_attacks_total` | Counter | `service`, `attack_type` | Total attacks dispatched per service and attack type |
| `chaos_arena_heals_total` | Counter | `service`, `outcome` | Total heal attempts; `outcome="healed"` means the reconciler confirmed recovery |
| `chaos_arena_mttr_milliseconds` | Gauge | `service` | Last observed mean-time-to-recover in ms for each service |
| `chaos_arena_services_healthy` | Gauge | — | Number of services currently passing their health check |

### Useful PromQL queries

Paste these into **Explore** (select the Prometheus datasource first):

```promql
# Total attacks by type across all services
sum by (attack_type) (chaos_arena_attacks_total)

# Successful heals per service
chaos_arena_heals_total{outcome="healed"}

# MTTR trend over time (requires multiple data points)
chaos_arena_mttr_milliseconds

# Services healthy right now
chaos_arena_services_healthy

# Attack rate over a sliding 5-minute window
rate(chaos_arena_attacks_total[5m])
```

### Building a dashboard

1. Go to **Dashboards → New → New dashboard → Add visualization**.
2. Select the Prometheus datasource.
3. Use the queries above as panel metrics.
4. Recommended panels:
   - **Stat** — `chaos_arena_services_healthy` (big number, green/red threshold at 3)
   - **Time series** — `chaos_arena_attacks_total` with `rate([1m])` to see attack bursts
   - **Bar chart** — `sum by (service) (chaos_arena_heals_total{outcome="healed"})` for per-service heal counts
   - **Stat** — `chaos_arena_mttr_milliseconds` per service to compare recovery speed

## Configuration

Edit `arena.yaml` to change the service topology, simulation duration, and scheduled auto-attacks.

```yaml
game:
  duration_seconds: 60
  auto_attacks:
    - at_second: 10
      service: service-b
      attack: kill_switch
```

Reports are written to `./reports/{game_id}.json`.
