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
| `POST` | `/game/start` | Start a session → `{ "game_id": "..." }` |
| `POST` | `/game/stop` | End session, write post-mortem → report JSON |
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

```bash
make grafana
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin / admin)

## Configuration

Edit `arena.yaml` to change the service topology, game duration, and scheduled auto-attacks.

```yaml
game:
  duration_seconds: 60
  auto_attacks:
    - at_second: 10
      service: service-b
      attack: kill_switch
```

Reports are written to `./reports/{game_id}.json`.
