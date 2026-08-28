# runpod-exporter

A Prometheus exporter for the [Runpod](https://runpod.io) v2 REST API. Scrapes and exports pod
utilization, serverless workers, billing, catalog pricing and more.

## Run

```
cp .env.example .env                            # fill in RUNPOD_API_KEY
docker compose up                                # exporter alone, or:
docker compose -f deploy/docker-compose.yml up   # full stack: exporter + Prometheus + Grafana
```

Both pull the published image, `evanofslack/runpod-exporter`.

Use a **read-only** Runpod API key, the exporter only ever makes `GET`
requests.

Metrics are served at `http://localhost:9836/metrics`. For the full stack,
open Grafana at `http://localhost:3000` (default login `admin`/`admin`) — the
"Runpod Exporter" dashboard is already loaded, no import needed.

## Configuration

Flags and env vars (flag wins if both are set):

| Flag                     | Env                           | Default                          |
| ------------------------ | ----------------------------- | -------------------------------- |
| `--api-key`              | `RUNPOD_API_KEY`              | required                         |
| `--api-url`              | `RUNPOD_API_URL`              | `https://api.runpod.io`          |
| `--domains`              | `RUNPOD_DOMAINS`              | `pod,account,billing` (or `all`) |
| `--listen-addr`          | `RUNPOD_LISTEN_ADDR`          | `:9836`                          |
| `--scrape-interval`      | `RUNPOD_SCRAPE_INTERVAL`      | `30s`                            |
| `--scrape-interval-slow` | `RUNPOD_SCRAPE_INTERVAL_SLOW` | `5m`                             |
| `--log-level`            | `RUNPOD_LOG_LEVEL`            | `info`                           |

Run with `--help` for details. See [plans/0001-v1-exporter.md](plans/0001-v1-exporter.md)
for the full metric catalog and design.

## Metrics

A sample — `--domains=all` exposes many more (serverless, cluster, catalog,
registry, template, network-volume); see the spec above for the full list.

```
# HELP runpod_pod_up 1 if the pod exists, 0 otherwise. Labeled with its current status.
# TYPE runpod_pod_up gauge
runpod_pod_up{pod_id="pod_abc123",pod_name="my-training-pod",status="RUNNING"} 1
# HELP runpod_pod_cpu_util_percent CPU utilization percent. Absent while the pod's runtime is null.
# TYPE runpod_pod_cpu_util_percent gauge
runpod_pod_cpu_util_percent{pod_id="pod_abc123"} 45
# HELP runpod_pod_gpu_util_percent Per-GPU utilization percent. Absent while the pod's runtime is null.
# TYPE runpod_pod_gpu_util_percent gauge
runpod_pod_gpu_util_percent{gpu_index="0",pod_id="pod_abc123"} 94
# HELP runpod_account_ssh_keys Number of SSH keys registered on the account.
# TYPE runpod_account_ssh_keys gauge
runpod_account_ssh_keys 1
# HELP runpod_billing_cost_dollars Latest hourly billing bucket's cost in USD, by resource.
# TYPE runpod_billing_cost_dollars gauge
runpod_billing_cost_dollars{resource="pod_gpu"} 0.44
```

## Development

Build and run from local source instead of the published image:

```
just build     # build the binary
just run       # go run, reads .env
just dev-up    # docker compose -f docker-compose-dev.yaml up --build
just dev-down
just test      # run tests
just vet       # go vet
just fmt       # gofmt
```
