# 0002 — monitoring stack: Grafana dashboard + docker-compose example

Status: in-progress (Stage 2 done — needs browser verification, see decisions log)
Scope: a docker-compose stack (prometheus + grafana + runpod-exporter) that anyone
cloning this repo can bring up and immediately see real dashboards, plus the Grafana
dashboard itself, committed as JSON and auto-provisioned — no manual import. Builds on
[0001-v1-exporter.md](0001-v1-exporter.md), which explicitly scoped this out as a
later spec.

## 1. Motivation

0001 shipped a working exporter with 9 domains' worth of metrics, but "run the binary
and curl /metrics" is not how anyone actually wants to look at this data day to day.
This spec turns the metric catalog into something you'd actually put on a screen:
a docker-compose stack that stands up the full pipeline (exporter → Prometheus →
Grafana) in one command, with a dashboard that's already there when Grafana starts —
committed to the repo, not hand-built through the UI and forgotten.

## 2. Goals

- `deploy/docker-compose.yml` — exporter + Prometheus + Grafana, one command to see
  real dashboards against a real account.
- Grafana auto-provisions its Prometheus datasource and the dashboard itself from
  files in the repo — zero manual clicking in the Grafana UI on first boot.
- One dashboard, `runpod-exporter`, with a row per domain (per the user's explicit
  decision — see §7 decisions log), styled per the user's established dashboard
  conventions (§4).
- Dashboard JSON is hand-maintained and committed like any other source file, not
  exported-and-forgotten from a UI session.

## 3. Non-goals

- No alerting rules, no Alertmanager. Purely visualization.
- No multi-account / multi-datasource support in the dashboard — one Prometheus
  datasource variable, as in the user's reference dashboard, but this stack assumes
  one exporter instance behind it.
- No dashboard for domains this repo doesn't implement (there are none — 0001 covers
  all 9).
- No production-grade Prometheus/Grafana deployment concerns (auth hardening,
  persistent storage tuning, HA). This is a "see your metrics" example stack, not a
  deployment guide.
- No CI validation of the dashboard JSON's correctness (e.g. no headless
  Grafana-renders-without-error check). Out of scope for v1 of this stack; the done
  when criteria in §8 are manual/visual, done by the user in a browser.

## 4. Dashboard conventions (from the user's reference dashboard)

Established by direct example in conversation, not to be reinterpreted per-panel:

- `$datasource` templating variable (type `datasource`, query `prometheus`),
  referenced as `${datasource}` in every panel and every variable's own datasource.
- Query variables for per-domain entity filters, multi-select, `includeAll: true`,
  sourced via `label_values(...)`. Scoped per-row, not global — see §5, no single
  cross-cutting entity variable exists here the way `$pod`/`$service` did in the
  reference (Runpod's metrics don't share label sets across domains).
- Row panels (`type: "row"`, `collapsed: false`) separating each domain, full width.
- `fieldConfig.defaults.min: 0` on every panel.
- Legend: `displayMode: "table"`, `placement: "right"`, calcs
  `["lastNotNull", "max"]`. `axisPlacement` stays `"auto"` (confirmed with the user —
  "axis on right" meant the legend, matching the reference exactly).
- `tooltip.mode: "multi"`.
- Short, plain panel titles ("CPU Utilization", not "Pod CPU Utilization Percent Over
  Time").
- `stat` panels for single current-value tiles (Overview row): `colorMode: "value"`,
  `graphMode: "area"` (sparkline), `reduceOptions.calcs: ["lastNotNull"]`.
- `timeseries` panels for anything trended: `drawStyle: "line"`, `fillOpacity: 10`
  (0 where multiple series would visually compete, e.g. per-pod overlays).
- `table` panels for reference/inventory data that isn't meaningfully a trend
  (catalog pricing snapshot, template/registry inventory).
- `refresh: "30s"`, `timezone: "utc"`, default time range `now-6h`.
- Fixed, human-readable dashboard `uid` (`runpod-exporter`), not Grafana's
  random-generated one — stable across re-provisioning.

## 5. Dashboard design

One dashboard, `runpod-exporter`. Rows in this order:

### Overview (stat tiles)

- **Pods Running** — `count(runpod_pod_up == 1)`
- **Pod Cost/hr** — `sum(runpod_pod_cost_per_hour_dollars)`
- **Billing Total This Hour** — `sum(runpod_billing_cost_dollars)`
- **Scrape Errors** — `sum(increase(runpod_exporter_scrape_errors_total[$__range]))`,
  red threshold at >0 — a health tile, not a metric tile.

No SSH-keys tile here (redundant with the Account row) — dropped per the user.

### Pod (`$pod_id`, multi, `label_values(runpod_pod_up, pod_id)`)

- **Pods by Status** (timeseries) —
  `sum by (status) (runpod_pod_up{pod_id=~"$pod_id"})`
- **CPU Utilization** (timeseries) —
  `runpod_pod_cpu_util_percent{pod_id=~"$pod_id"}`, legend `{{pod_id}}`
- **Memory Utilization** (timeseries) —
  `runpod_pod_memory_util_percent{pod_id=~"$pod_id"}`
- **GPU Utilization** (timeseries) —
  `runpod_pod_gpu_util_percent{pod_id=~"$pod_id"}`, legend `{{pod_id}} gpu{{gpu_index}}`
- **GPU Memory Utilization** (timeseries) — same shape as above,
  `runpod_pod_gpu_memory_util_percent`
- **Cost/hr** (timeseries) —
  `runpod_pod_cost_per_hour_dollars{pod_id=~"$pod_id"}`
- **Uptime** (stat, duration-formatted unit) —
  `runpod_pod_uptime_seconds{pod_id=~"$pod_id"}`

### Serverless (`$endpoint_id`, multi, `label_values(runpod_serverless_info, endpoint_id)`)

- **Workers by State** (stacked timeseries) —
  `sum by (endpoint_id, state) (runpod_serverless_workers{endpoint_id=~"$endpoint_id"})`
- **Stale Workers** (timeseries) —
  `runpod_serverless_worker_stale{endpoint_id=~"$endpoint_id"}`
- **Workers Min/Max** (timeseries, two queries) —
  `runpod_serverless_workers_min{endpoint_id=~"$endpoint_id"}` (legend
  `min - {{endpoint_id}}`) and `..._max` (legend `max - {{endpoint_id}}`)

### Billing (no variable — account-wide)

- **Cost by Resource** (stacked timeseries) — `runpod_billing_cost_dollars`, legend
  `{{resource}}`. Not a `rate()`/`increase()` — this is already a $/hr gauge, not a
  counter (unlike the reference dashboard's CDR counters).
- **Total Cost This Hour** (stat) — `sum(runpod_billing_cost_dollars)`

### Cluster (`$cluster_id`, multi, `label_values(runpod_cluster_info, cluster_id)`)

- **Cluster Pods by Status** (timeseries) —
  `sum by (cluster_id, status) (runpod_cluster_pods{cluster_id=~"$cluster_id"})`
- **Cluster Info** (table, instant) —
  `runpod_cluster_info{cluster_id=~"$cluster_id"}`

### Catalog (`$catalog_gpu_id`, multi, `label_values(runpod_catalog_gpu_price_dollars_per_hour, gpu_id)`)

Both trend panels are filtered by `$catalog_gpu_id` (unfiltered, dozens of GPU types ×
data centers × products is unreadable — confirmed dozens of series in the user's own
live output). The summary table below is deliberately **not** filtered by it — it's a
full reference lookup, meant to show everything at a glance.

- **GPU Price Trend** (timeseries) —
  `runpod_catalog_gpu_price_dollars_per_hour{gpu_id=~"$catalog_gpu_id"}`, legend
  `{{gpu_id}} ({{cloud}})`
- **GPU Availability Trend** (timeseries) —
  `runpod_catalog_gpu_availability{gpu_id=~"$catalog_gpu_id"}`, legend
  `{{gpu_id}} {{data_center_id}} ({{product}})`. Flagged as a real risk: even
  filtered to one GPU this can be 6+ series (data_center × product). If it renders
  unreadably in practice, split into per-product panels — decide after actually
  looking at it live, not preemptively.
- **GPU Price & Availability** (table, instant) — one row per `gpu_id`, three queries
  joined on `gpu_id` (Grafana "Join by field" transform), each pre-reduced to exactly
  one value per GPU so the join can't cross-multiply:
  - `max by (gpu_id) (runpod_catalog_gpu_price_dollars_per_hour{cloud="SECURE"})` →
    column "Secure $/hr"
  - `max by (gpu_id) (runpod_catalog_gpu_price_dollars_per_hour{cloud="COMMUNITY"})` →
    column "Community $/hr"
  - `max by (gpu_id) (runpod_catalog_gpu_availability)` → column "Best Availability"
    (value-mapped 0/1/2/3 → NONE/LOW/MEDIUM/HIGH for readability)

  This is the "combined into one table" the user asked for, achieved by reducing
  each side to a single value per GPU rather than a literal raw join — the raw label
  sets (`{gpu_id,cloud}` vs `{gpu_id,data_center_id,product}`) don't share a common
  key, so a naive join would cross-multiply into a confusing many-to-many table. Full
  per-cloud/per-datacenter/per-product granularity is still available in the two
  trend panels above and in `/metrics` directly — this table is a summary, not a
  replacement.
- **CPU Price per vCPU** (table, instant) — same two-query-join pattern:
  `max by (cpu_id) (runpod_catalog_cpu_price_dollars_per_vcpu_hour{cloud="SECURE"})`
  → "Secure $/vCPU/hr", `...{cloud="SERVERLESS"}` → "Serverless $/vCPU/hr".

### Account (no variable — single scalar)

- **SSH Keys** (stat) — `runpod_account_ssh_keys`

### Registry (no variable)

- **Registries** (stat) — `count(runpod_registry_info)`
- **Delegations** (stat) — `runpod_registry_delegation_count`

### Template (no variable)

- **Templates** (table, instant) — `runpod_template_info`

### Network Volume (no variable)

- **Volume Sizes** (table, instant) — `runpod_network_volume_size_gb`

## 6. Stack layout

```
deploy/
  docker-compose.yml                          - exporter + prometheus + grafana
  prometheus/
    prometheus.yml                            - scrape config, 15s interval
  grafana/
    provisioning/
      datasources/prometheus.yml              - auto-provisioned Prometheus datasource
      dashboards/dashboards.yml               - provider config, points at ./dashboards
    dashboards/
      runpod-exporter.json                    - the dashboard itself, committed
```

Deliberately separate from the existing root `docker-compose.yml` (dev-only, exporter
alone) — this is the full example stack, distinct purpose and audience. The 0001
decisions log already anticipated this split when the dev compose was built.

- Exporter service: `build: ..` from the repo's own `Dockerfile` (same as the dev
  compose — self-contained, no Docker Hub dependency), reads `.env` from repo root.
- Prometheus and Grafana: pinned to specific recent versions, not `:latest` — avoids
  surprise breakage. (Pin the exact tags at implementation time to whatever's current
  then, not baked into this spec.)
- Prometheus `scrape_interval: 15s`, independent of the exporter's own internal poll
  tiers (30s fast / 5m slow) — Prometheus is just sampling whatever's currently
  cached; scraping faster than the underlying data changes is harmless, just slightly
  redundant for slow-tier metrics.
- No persistent volumes for Prometheus/Grafana data — ephemeral is fine for an
  example stack. The dashboard itself persists because it's provisioned from a
  committed file, not because Grafana's own storage survives a restart.

## 7. Testing strategy

Everything here is YAML/JSON, not Go — the existing `internal/*` test suite is
unaffected and untouched by this spec. What can be verified without a browser:

- **Automatable, verify in CI or locally:** `docker compose -f deploy/docker-compose.yml
  config` validates the compose file parses; `docker compose up` brings up all three
  services; Prometheus's own `/-/healthy` and Grafana's `/api/health` endpoints
  respond; Prometheus's target-health API confirms it's successfully scraping the
  exporter; the dashboard JSON is valid JSON and (via Grafana's HTTP API,
  `GET /api/dashboards/uid/runpod-exporter`) confirms it auto-provisioned without a
  manual import.
- **Cannot be automated here, needs the user in an actual browser:** does each panel
  render sensibly, are the joins/transforms in the catalog table actually producing
  the intended shape, is the availability trend panel legible or overplotted, do the
  variables actually filter what they're supposed to. Same limitation as 0001's live
  API checks — this environment has no way to visually render and inspect a Grafana
  dashboard. Each stage's "done when" in §8 that says "renders correctly" means the
  user confirms it in a browser, not something claimed as verified without that.

## 8. Stages

- **Stage 0 — stack skeleton.** `deploy/docker-compose.yml`, `prometheus.yml`,
  Grafana datasource provisioning. No dashboard yet.
  *Done when:* `docker compose -f deploy/docker-compose.yml up` brings up all three
  services; Prometheus shows the exporter target as healthy; Grafana's Prometheus
  datasource test succeeds; a manual PromQL query in Grafana Explore returns real
  exporter metrics.

- **Stage 1 — dashboard skeleton + Overview/Pod/Serverless rows.** Dashboard
  provisioning wired up, `runpod-exporter.json` committed with the `$datasource`
  variable, Overview row, and the two richest domain rows.
  *Done when:* the dashboard is present in Grafana on startup with no manual import;
  user confirms in a browser that Overview/Pod/Serverless rows render correctly
  against a live account.

- **Stage 2 — Billing/Cluster/Catalog rows.** Includes the catalog join-table design
  from §5 — the highest-risk panel in this spec, most likely to need iteration once
  actually rendered.
  *Done when:* user confirms these three rows render correctly, including the
  combined GPU price/availability table actually joining as intended.

- **Stage 3 — Account/Registry/Template/Network-Volume rows.** The thin domains,
  mechanical repeats of the table/stat patterns already established.
  *Done when:* user confirms all four rows render correctly.

- **Stage 4 — polish.** README section on running the full stack
  (`docker compose -f deploy/docker-compose.yml up`, what URL to open, default
  Grafana credentials). Finalize dashboard `tags`/`description`.
  *Done when:* a fresh clone of the repo can go from `git clone` to a working
  dashboard using only the README's instructions.

## 9. Decisions log

- Scope: one row per domain, all 9, sized to what each domain actually has — not
  narrowed to just the rich domains. Confirmed with the user.
- No single cross-cutting entity variable like the reference dashboard's `$pod`/
  `$service` — Runpod's metrics don't share label sets across domains, so variables
  are scoped per-row (`$pod_id`, `$endpoint_id`, `$cluster_id`, `$catalog_gpu_id`).
  Thin domains (account, registry, template, network-volume) get no variable at all —
  table/single-stat panels don't need one.
- "Axis on right" in the user's dashboard-style request meant the *legend*
  (`placement: "right"`), not `axisPlacement` — confirmed against the reference
  dashboard, which uses `axisPlacement: "auto"` throughout despite the legend being
  on the right.
- Billing and catalog panels deviate from the reference dashboard's "everything is
  `rate()`/`increase()`" pattern: billing's `runpod_billing_cost_dollars` and
  catalog's pricing/availability metrics are already-computed gauges (a $/hr rate, a
  price, a stock level), not counters — wrapping them in `rate()` would be wrong.
- GPU price and availability get **both** a trend (timeseries) and a current-snapshot
  reference table, per the user — dropping the earlier assumption that catalog data
  was purely "reference/lookup, not worth trending."
- The combined GPU price+availability table doesn't do a literal join of the raw
  metrics (their label sets don't share a common key — `{gpu_id,cloud}` vs.
  `{gpu_id,data_center_id,product}` — a naive join would cross-multiply). Instead each
  side is pre-reduced to one value per `gpu_id` (`max by (gpu_id) (...)`, filtered to
  a specific `cloud`/taking the best availability across all contexts) before
  joining, trading per-dimension granularity in *this one summary panel* for a clean
  table — full granularity stays available in the trend panels. This is what the user
  meant by "see if it can be combined... but don't force it if not easy" — it can be,
  via aggregation, not a raw join.
- New `deploy/` directory, distinct from the existing root `docker-compose.yml`
  (dev-only, exporter alone) — anticipated in 0001's own decisions log when the dev
  compose was built.
- Exporter service builds from the local `Dockerfile`, not a published Docker Hub
  image — self-contained for anyone cloning the repo, no registry dependency, matches
  the existing dev compose's approach.
- Dashboard rendering correctness is explicitly *not* something claimed as verified
  by the agent implementing this spec — no browser/visual tool is available in that
  environment. Every stage's done-when criteria that involves "renders correctly"
  requires the user's own visual confirmation, same limitation already established
  for 0001's live API checks.
- **Stage 0:** pinned to the actual latest stable releases at implementation time —
  Prometheus `v3.14.0`, Grafana `13.2.0` (checked via `gh api
  repos/{prometheus,grafana}/{prometheus,grafana}/releases/latest`, not guessed).
  `depends_on` uses plain start-order, not `condition: service_healthy` — the
  exporter's own `/healthz` exists for exactly this, but the Dockerfile's distroless
  base image has no shell/curl to run a container `HEALTHCHECK` with, and switching
  base images or shipping a separate healthcheck binary is more complexity than this
  stage needs. Prometheus's first scrape or two may show the target down until the
  exporter's HTTP server is ready; it self-heals within one 15s scrape interval.
  **Not verified against real `docker compose`** — no `docker` CLI/daemon in this
  environment (same limitation as 0001 Stage 5). All four YAML files were validated
  for syntax (`ruby -ryaml`), and the compose/provisioning structure matches
  Grafana's and Prometheus's standard, long-stable documented formats, but actually
  bringing the stack up and confirming Prometheus scrapes the exporter and Grafana's
  datasource test succeeds is unverified — needs the user to run it.
- Stage 0 confirmed working by the user — all three services came up, Prometheus
  scraped the exporter, Grafana's datasource test passed, Explore returned real data.
- **Stage 1:** `deploy/grafana/dashboards/runpod-exporter.json` generated via a
  throwaway Python script (dict construction, `json.dump`) rather than hand-typed —
  17 panels of mostly-repeated boilerplate is much less error-prone built
  programmatically than typed by hand. The script itself isn't committed, only its
  output; regenerating it isn't a normal workflow (further dashboard changes are
  direct edits to the committed JSON, same as any other source file).
  `fillOpacity: 0` (lines only, no fill) on every panel keyed by a multi-select
  per-entity variable (Pod row's utilization/cost panels, Serverless's stale/min-max
  panels) — per §4, avoids a messy shaded look when several pods/endpoints are
  selected and their lines overlap. `fillOpacity: 10` stays on the small
  fixed-cardinality "by status" panel. "Workers by State" uses `stacking.mode:
  "normal"` with `fillOpacity: 20` (a stacked area, not overlaid lines) since spec
  §5 called it out as stacked specifically.
  **Not verified in a browser** — same limitation as Stage 0 and 0001's live checks.
  Validated: JSON parses, panel `id`s are unique, no two panels' `gridPos` rectangles
  overlap (checked programmatically). Not validated: whether it actually renders
  sensibly, whether `$pod_id`/`$endpoint_id` actually populate from real label
  values, whether the stat panels' multi-series-per-tile behavior looks right —
  needs the user in an actual Grafana UI.
- Stage 1 confirmed working by the user.
- **Stage 2:** the combined GPU price+availability table works exactly as designed
  in §5 — three instant, table-format queries (each pre-reduced to one row per
  `gpu_id` via `max by (gpu_id) (...)`) joined with Grafana's `joinByField`
  transform (`mode: "outer"`, so a GPU missing from one query's result still shows
  up with a blank cell rather than being dropped), then an `organize` transform
  renames the resulting `Value #A`/`#B`/`#C` columns to their real meaning and drops
  `Time` (meaningless for an instant snapshot). Same pattern for the CPU price
  table. `Best Availability`'s 0–3 values get a field-override value mapping to
  NONE/LOW/MEDIUM/HIGH text, via a `byName` matcher.
  Caught and fixed before committing: the `$catalog_gpu_id` variable's own
  `label_values(...)` query initially used `catalog_gpu_id` as the label name
  (copied from the Grafana variable's name) instead of the real Prometheus label,
  `gpu_id` — variable name and label name only coincide for `pod_id`/`endpoint_id`/
  `cluster_id` because those weren't given a domain-prefixed name; `catalog_gpu_id`
  was, specifically to avoid confusion with `pod_id`'s own `gpu_id`-shaped label on
  `runpod_pod_info`, and that rename broke the generic panel-generation helper's
  assumption that they're always the same string. Fixed by decoupling the
  variable's display name from the label name it actually queries.
  **Not verified in a browser** — same limitation as every prior stage. This is the
  riskiest panel in the whole spec (§8 called it out specifically); the join/rename/
  value-mapping chain is exactly the kind of thing that looks right in the JSON and
  still renders wrong, so it genuinely needs to be looked at, not just trusted.
