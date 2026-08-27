# runpod-exporter

A Prometheus exporter for the [Runpod](https://runpod.io) v2 REST API — pod
utilization, serverless workers, billing, and catalog pricing.

## Run

```
cp .env.example .env   # fill in RUNPOD_API_KEY
just dev-up            # docker compose, or:
just run               # go run, from source
```

Metrics are served at `http://localhost:9836/metrics`, health at `/healthz`.

## Configuration

Flags and env vars — flag wins if both are set:

| Flag | Env | Default |
|---|---|---|
| `--api-key` | `RUNPOD_API_KEY` | required |
| `--api-url` | `RUNPOD_API_URL` | `https://api.runpod.io` |
| `--domains` | `RUNPOD_DOMAINS` | `pod,account,billing` (or `all`) |
| `--listen-addr` | `RUNPOD_LISTEN_ADDR` | `:9836` |
| `--scrape-interval` | `RUNPOD_SCRAPE_INTERVAL` | `30s` |
| `--scrape-interval-slow` | `RUNPOD_SCRAPE_INTERVAL_SLOW` | `5m` |
| `--log-level` | `RUNPOD_LOG_LEVEL` | `info` |

Run with `--help` for details. See [plans/0001-v1-exporter.md](plans/0001-v1-exporter.md)
for the full metric catalog and design.

## Development

```
just build     # build the binary
just run       # go run, reads .env
just dev-up    # docker compose up --build
just dev-down  # docker compose down
just test      # run tests
just vet       # go vet
just fmt       # gofmt
```
