# 0001 — runpod-exporter v1

Status: in-progress (Stage 1 done)
Scope: a standalone Prometheus exporter binary for the Runpod v2 REST API. This spec
covers the exporter application only. Grafana dashboards and a docker-compose
(prometheus + grafana + exporter) stack are a separate, later spec that builds on
this one once v1 is running.

## 1. Motivation

The Runpod console shows live per-pod CPU/GPU/memory utilization. That data has no
scrape-able source today. Runpod's v2 REST API (`api.runpod.io/v2`, described by its
own `openapi.json`) exposes it directly on `Pod.runtime` along with billing, serverless
worker state, and catalog/pricing data across nine resource domains. The only prior
art (`runpod-general-prometheus-textfile-exporter`) is a cron + textfile-collector
script against the GraphQL API that reports only account-level scalars (balance,
spend/hr, resource counts) — no per-resource labels, no utilization, no cost
breakdown. This project is a proper long-running exporter against the richer v2 REST
surface.

## 2. Goals

- One long-running Go binary. Poll Runpod v2 REST on a per-domain interval, serve
  `/metrics` in Prometheus exposition format.
- Cover all nine domains the openapi spec itself groups resources into: `pod`,
  `serverless`, `billing`, `account`, `cluster`, `template`, `network-volume`,
  `registry`, `catalog`. Enable a subset via flag/env; default to `pod,account,billing`.
- Metric names and labels track the API's own field names/enums directly — no
  invented vocabulary.
- Config entirely via flags and env vars (env is the default source, flag overrides).
- Graceful shutdown on SIGINT/SIGTERM.

## 3. Non-goals (v1)

- No client-side rate limiting / token bucket. Per-domain intervals plus the
  documented 180 req/min budget give enough headroom for normal fleet sizes; a
  bursty domain (`serverless`, N+1 across many endpoints) is a v2 concern if it
  ever comes up.
- No retry/backoff beyond "log and wait for the next tick." A transient error
  (429, 5xx, timeout) is not retried mid-cycle.
- No auth in front of `/metrics`. Standard exporter assumption — put it behind
  your own network boundary/reverse proxy.
- No multi-account support (one API key per process).
- No historical backfill of billing data — only the latest bucket per poll.
- No Grafana dashboard JSON, no docker-compose stack. Next spec.
- No `account` domain beyond ssh-key count — v2 REST has no balance/spend-rate
  endpoint (that was GraphQL-only, `myself.clientBalance` etc.); `/v2/account`
  in this API is `ssh-keys` only.

## 4. Architecture

```
cmd/runpod-exporter/main.go   - flag/env parsing, wiring, signal handling, http server
internal/config/              - flag+env resolution, domain-set + interval validation
internal/collector/           - one file per domain: pod.go, serverless.go, billing.go, ...
internal/collector/domain.go  - Domain interface, shared poll-loop runner
internal/metrics/             - prometheus metric descriptors (GaugeVec/CounterVec per domain)
openapi/                      - generated client only (oapi-codegen), nothing else imports it
                                 except internal/collector/*
plans/                        - this spec and future ones
justfile
go.mod
```

**Poll-cache-serve, not collect-on-scrape.** Each enabled domain runs its own
background loop on its own ticker, calls the API, and updates that domain's
`prometheus.GaugeVec`/`CounterVec` in place. `/metrics` (via `promhttp.Handler()`)
just serves whatever's currently in the registry — it never blocks on a live API
call. This is what makes the "stale-serve on error" policy in §6 free: a failed
poll simply doesn't update the vecs, so the last good values keep being served
until the next successful poll.

**Vanished-resource handling.** Each domain's vecs are `Reset()` immediately before
repopulating from a successful poll response (not before a failed one). A pod that
gets terminated between polls disappears from the API list and therefore from
`/metrics` on the next successful poll, rather than reporting a frozen last-known
value forever. This is a real behavior to unit test, not an implementation detail.

**Generated client boundary.** `openapi/` is regenerated via `go generate` (oapi-codegen
against a vendored copy of `openapi.json`) and is never hand-edited. Only
`internal/collector/*` imports it — no other package talks to Runpod directly.

## 5. Domain × metric catalog

Interval tier: **fast** = default `--scrape-interval`, applies to domains with
state that changes second-to-second. **slow** = `--scrape-interval-slow`, applies
to near-static/reference/aggregate domains where polling on the fast tier would
just be waste.

| Domain | Tier | Source calls |
|---|---|---|
| pod | fast | `GET /v2/pods` (runtime is inline, single call) |
| serverless | fast | `GET /v2/serverless`, then `GET /v2/serverless/{id}/workers` per endpoint |
| account | fast | `GET /v2/account/ssh-keys` |
| cluster | fast | `GET /v2/clusters` |
| billing | slow | `GET /v2/billing`, `/v2/billing/pods`, `/v2/billing/serverless` (`bucketSize=hour&lastN=1`) |
| catalog | slow | `GET /v2/catalog/gpus`, `/cpus`, `/datacenters` |
| template | slow | `GET /v2/templates` |
| registry | slow | `GET /v2/registries`, `/v2/registries/delegations` |
| network-volume | slow | `GET /v2/network-volumes` |

### pod (fast)

- `runpod_pod_up{pod_id,pod_name,status}` — 1/0. `status` is `Pod.status`
  (`PROVISIONING|STARTING|RUNNING|EXITED|ERROR|TERMINATED`). Doubles as the
  "container not up yet" signal: utilization series are simply absent while
  `runtime == null`.
- `runpod_pod_cpu_util_percent{pod_id}` — `runtime.cpu.util`
- `runpod_pod_memory_util_percent{pod_id}` — `runtime.memory.util`
- `runpod_pod_gpu_util_percent{pod_id,gpu_index}` — `runtime.gpus[i].util`
- `runpod_pod_gpu_memory_util_percent{pod_id,gpu_index}` — `runtime.gpus[i].memoryUtil`
- `runpod_pod_uptime_seconds{pod_id}` — `runtime.uptime`
- `runpod_pod_cost_per_hour_dollars{pod_id}` — `Pod.cost`
- `runpod_pod_disk_gb{pod_id}` — `Pod.disk`
- `runpod_pod_info{pod_id,image,data_center_id,cloud,gpu_id}` — value 1, standard info-metric pattern

### serverless (fast)

- `runpod_serverless_workers{endpoint_id,state}` — from `WorkerSummary`, `state ∈
  running|idle|initializing|throttled|unhealthy`
- `runpod_serverless_worker_stale{endpoint_id}` — count of `Worker.isStale`
- `runpod_serverless_workers_min{endpoint_id}` / `runpod_serverless_workers_max{endpoint_id}` — `EndpointWorkers.min/max`
- `runpod_serverless_info{endpoint_id,name,type,flashboot}` — `type ∈ QUEUE|LOAD_BALANCER`

### account (fast)

- `runpod_account_ssh_keys` — `len(SshKeys.keys)`

### cluster (fast)

- `runpod_cluster_pods{cluster_id,status}` — `ClusterPodsSummary.byStatus`
- `runpod_cluster_info{cluster_id,type,data_center_id}` — `type ∈ APPLICATION|TRAINING|SLURM|RAY`

### billing (slow)

- `runpod_billing_cost_dollars{resource}` — latest hourly bucket, gauge (no
  bucket-time label — the trend comes from Prometheus's own scrape timestamps).
  `resource ∈ pod_gpu|pod_cpu|pod_disk|serverless_gpu|serverless_cpu|serverless_disk|
  serverless_fee|storage_standard|storage_high_performance|endpoint|cluster_gpu|
  cluster_disk|cluster_networking` — directly the `BillingAmounts` field names.

### catalog (slow)

- `runpod_catalog_gpu_price_dollars_per_hour{gpu_id,cloud}` — `GpuType.price`, `cloud ∈ secure|community`
- `runpod_catalog_cpu_price_dollars_per_hour{cpu_id}` — `CpuType.price`
- `runpod_catalog_gpu_availability{gpu_id,data_center_id}` — `AvailabilityLevel` enum, mapped to int

### template (slow)

- `runpod_template_info{template_id,name,serverless,public,category}` — inventory only

### registry (slow)

- `runpod_registry_info{registry_id,name}`
- `runpod_registry_delegation_count`

### network-volume (slow)

- `runpod_network_volume_size_gb{volume_id,data_center_id,type}` — `type ∈ STANDARD|HIGH_PERFORMANCE`

### exporter self-observability (always on, regardless of enabled domains)

- `runpod_exporter_scrape_errors_total{domain}` — counter
- `runpod_exporter_last_success_timestamp_seconds{domain}` — gauge
- `runpod_exporter_scrape_duration_seconds{domain}` — gauge (last poll's wall time; no histogram in v1, keep it simple)

## 6. Collector contract & error policy

```go
// internal/collector
type Domain interface {
	Name() string           // "pod", "serverless", ...
	Tier() Tier              // Fast or Slow
	Poll(ctx context.Context) error // updates this domain's metric vecs in place
}
```

A single runner in `internal/collector/domain.go` takes the enabled `[]Domain`,
starts one goroutine per domain on its own `time.Ticker` (interval from its
`Tier()`), and on each tick:

1. Call `Poll(ctx)` with a per-call timeout derived from `ctx` (bounded, so one
   hung request can't wedge that domain's loop forever).
2. On success: reset the domain's vecs, populate from the response, set
   `last_success_timestamp` and `scrape_duration`.
3. On error: log it (structured, see §9), increment `scrape_errors_total`, leave
   existing vec values untouched (stale-serve — this is the whole point of
   decoupling polling from serving). Do not reset on error, or a transient
   failure would blank the domain until the next successful poll instead of
   just going stale.

HTTP status handling lives in the collector, not the generated client: a non-2xx
response is wrapped into an error that includes the status code and a truncated
body, so `Poll`'s error path can distinguish "auth/config problem" (4xx — still
just logged and stale-served in v1, no special-casing beyond a clearer message)
from "transient" (429/5xx) without needing retry logic to act on the distinction.

## 7. Configuration

Flags, each with a matching env var; flag wins if both are set. Defaults are
resolved from env before flag registration so `--help` shows the effective value.

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--api-key` | `RUNPOD_API_KEY` | — | required |
| `--api-url` | `RUNPOD_API_URL` | `https://api.runpod.io/v2` | |
| `--domains` | `RUNPOD_DOMAINS` | `pod,account,billing` | comma list or `all` |
| `--listen-addr` | `RUNPOD_LISTEN_ADDR` | `:9836` | |
| `--scrape-interval` | `RUNPOD_SCRAPE_INTERVAL` | `30s` | fast-tier domains |
| `--scrape-interval-slow` | `RUNPOD_SCRAPE_INTERVAL_SLOW` | `5m` | slow-tier domains only |
| `--log-level` | `RUNPOD_LOG_LEVEL` | `info` | debug/info/warn/error |

Validation at startup (fail fast, before the HTTP server starts listening):
unknown domain name in `--domains`, empty `--api-key`, interval below a sane
floor (5s), unparseable `--api-url`.

## 8. Concurrency, context, and shutdown

- Every function that makes an API call takes `context.Context` as its first
  argument. No package-level/background contexts inside `internal/collector`.
- `main.go` builds its root context with `signal.NotifyContext(context.Background(),
  os.Interrupt, syscall.SIGTERM)`. That context is threaded into every domain's
  poll loop and into the `http.Server`.
- On cancellation: stop accepting new ticks (domain loops select on `ctx.Done()`
  between ticks, not mid-request — an in-flight poll finishes or hits its own
  per-call timeout), then `http.Server.Shutdown(shutdownCtx)` with a bounded
  timeout (5s) so an in-flight scrape from Prometheus is allowed to finish.

## 9. Error handling & logging conventions

- Wrap at every boundary with `fmt.Errorf("...: %w", err)` so the root cause
  (including a wrapped HTTP status error) survives to the log call.
- `log/slog`, structured, snake_case field keys — e.g.
  `slog.Error("poll failed", "domain", "pod", "status_code", 429, "elapsed_ms", 812)`.
  One handler configured once in `main.go` from `--log-level`; `slog.SetDefault`.
- No comments unless the *why* isn't obvious from the code — this applies to
  generated-vs-hand-written boundaries too (a one-liner on `openapi/` saying
  "generated, do not edit" is the kind of comment that's actually earning its
  place; restating what a `Poll` method does is not).

## 10. Testing strategy

Stdlib only: `testing` + table-driven tests + `net/http/httptest.Server` for
fixture servers. Test what has logic, skip what's wiring:

- **Test:** `internal/config` — flag/env precedence, domain-set validation,
  interval floor validation, `all` expansion.
- **Test:** each `internal/collector/*` — `Poll` against an `httptest.Server`
  serving a fixture JSON body (seeded from the real openapi.json examples/live
  responses already captured while scoping this): correct metric values/labels
  on success, `scrape_errors_total` increments and stale values persist on a
  500/429 fixture, vecs actually shrink (vanished-resource case) when a second
  poll's fixture has fewer resources than the first.
- **Skip:** `cmd/runpod-exporter/main.go` (pure wiring), `internal/metrics`
  descriptor definitions (no logic, just registration), the generated `openapi/`
  client itself.

## 11. Dev tooling — justfile

```
just build      # go build ./cmd/runpod-exporter
just run        # go run ./cmd/runpod-exporter (reads .env if present)
just test       # go test ./...
just vet        # go vet ./...
just fmt        # gofmt -l -w .
just generate   # go generate ./openapi
just tidy       # go mod tidy
```

## 12. CI (placeholder)

GitHub Actions is the final stage of v1, not built until an example workflow is
provided to mirror. Requirements to fold in when that happens: run `test` + `build`
on every PR; on a release, additionally build and push a container image.

## 13. Stages

Each stage should be independently mergeable, with acceptance criteria stated
as something checkable.

- **Stage 0 — scaffold.** `go.mod`, `openapi/` generated client + `go:generate`
  directive committed, bare binary serving `/healthz` and an empty `/metrics`,
  flag/env parsing wired, justfile.
  *Done when:* `just build && just run` serves both endpoints; `just test` passes
  (config tests only at this point).

- **Stage 1 — pod domain.** Full `pod` collector: metrics, vec reset/error policy,
  self-observability metrics. This stage sets the pattern every later domain copies.
  *Done when:* against a live pod, `runpod_pod_cpu_util_percent` etc. show real
  non-zero values while running and disappear (pod still listed, but no util
  series) when stopped; `runpod_pod_up` reflects status either way; collector
  unit tests pass against fixtures.

- **Stage 2 — account + billing.** Default domain set (`pod,account,billing`)
  complete.
  *Done when:* default-flag run exposes all three domains' metrics against a
  live account.

- **Stage 3 — serverless.** Establishes the N+1 (list, then per-resource detail)
  pattern.
  *Done when:* `runpod_serverless_workers` state counts match `serverless get`
  output for a live endpoint.

- **Stage 4 — remaining domains.** `cluster`, `network-volume`, `template`,
  `registry`, `catalog` — mechanical repeats of the Stage 1/3 pattern.
  *Done when:* `--domains=all` runs clean against a live account with every
  domain's metrics present.

- **Stage 5 — packaging.** Dockerfile, README, GitHub Actions CI per §12.
  *Done when:* `docker build` succeeds and the resulting image serves `/metrics`
  identically to `go run`.

## 14. Decisions log

- No token bucket / rate limiter in v1 — per-domain intervals plus the 180
  req/min budget are enough headroom; revisit only if a domain's N+1 fan-out
  becomes a real problem at scale.
- Two-tier interval model (`--scrape-interval` fast default, `--scrape-interval-slow`
  override), not a flag per domain — simpler surface, and only the genuinely
  near-static domains need the override.
- `account` domain stays ssh-key-count-only for v1; v2 REST has no
  balance/spend endpoint to report beyond that.
- New standalone repo (this one), not a subdirectory of `runpodctl` — different
  binary, different audience (ops/server vs CLI).
- Stale-serve on poll error (not drop-series), paired with per-domain
  `scrape_errors_total` / `last_success_timestamp` so staleness is something
  Prometheus itself can alert on.
- Grafana dashboards + docker-compose (prometheus + grafana + exporter) stack
  are their own spec, written after v1 is running end to end.
- `--api-key`'s default is left blank in flag registration (unlike every other
  flag) and applied from `RUNPOD_API_KEY` only after `flag.Parse`, so `--help`
  never echoes a secret pulled from the environment.
- oapi-codegen's default `{OperationId}Response` wrapper name collides with
  several of this API's own schema names (e.g. `ListBillingResponse` is both
  a response-envelope struct and a real schema). Fixed via
  `output-options.response-type-suffix: HTTPResponse` in `oapi-codegen.yaml`.
- Per-call poll timeout is a flat 15s constant (`pollTimeout` in
  `internal/collector/domain.go`) — the spec left the exact value open.
- A configured domain with no registered collector (e.g. `account`/`billing`
  before their stages land) logs a warning and is skipped rather than
  failing startup, so the default `--domains` flags stay usable stage by
  stage. Decided with the user ahead of Stage 1.
- The self-observability metrics (`scrape_errors_total`,
  `last_success_timestamp_seconds`, `scrape_duration_seconds`) and the
  poll-error/stale-serve mechanics live in the shared runner
  (`internal/collector/domain.go`), not in each domain's own `Poll` — every
  domain gets this for free, so it's tested once (`domain_test.go` with a
  fake `Domain`) rather than re-verified per domain.
- A domain's per-call context is derived from `context.Background()` with
  just `pollTimeout` applied, not from the runner's cancellable `ctx` —
  otherwise a shutdown signal would abort an in-flight poll mid-request,
  contradicting §8's "an in-flight poll finishes... not mid-request."
  Shutdown only takes effect between ticks.
- That grace isn't unbounded: if shutdown begins while a poll is in flight,
  it now gets `shutdownGrace` (5s, matching the HTTP server's own shutdown
  budget) to finish on its own before `poll()` force-cancels it. Implemented
  as a monitor goroutine per poll that `poll()` explicitly joins before
  returning — it does not outlive the call, so it can't race a later poll's
  (or test's) read of the timing vars. Covered by
  `TestPoll_ForceCancelsAfterShutdownGrace` and
  `TestPoll_FinishesWithinGraceWithoutForceCancel`, both passing under
  `-race`.
