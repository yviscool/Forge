# Observability and Operations

## HTTP metrics

Forge exposes dependency-free Prometheus text metrics at `GET /metrics`.
The endpoint never includes contest IDs, user IDs, submission IDs, request
bodies, or credentials. Dynamic paths are normalized to bounded route labels
such as `/api/v1/contests/:cid/submissions`.

Available metrics:

- `forge_http_requests_in_flight`
- `forge_http_requests_total{method,route,status}`
- `forge_http_request_duration_seconds{method,route,status}`
- `forge_judge_workers`
- `forge_judge_jobs_active`
- `forge_judge_jobs_queued`
- `forge_judge_queue_capacity`
- `forge_judge_enqueue_failures_total`
- `forge_sse_subscribers`

The HTTP access log includes `method`, `path`, `status`, `traceId`, and
`dur`. Every response also carries `X-Trace-Id`.

## Prometheus scrape

Keep `/metrics`, `/healthz`, and `/readyz` reachable only from the loopback
interface or the monitoring network. Do not expose them as a public Internet
route through the reverse proxy.

```yaml
scrape_configs:
  - job_name: forge
    metrics_path: /metrics
    static_configs:
      - targets: ["127.0.0.1:8080"]
```

Example alert rules:

```yaml
groups:
  - name: forge
    rules:
      - alert: ForgeDown
        expr: up{job="forge"} == 0
        for: 2m

      - alert: ForgeHigh5xxRate
        expr: |
          sum(rate(forge_http_requests_total{job="forge",status=~"5.."}[5m]))
          /
          clamp_min(sum(rate(forge_http_requests_total{job="forge"}[5m])), 0.001)
          > 0.02
        for: 5m

      - alert: ForgeSlowAPI
        expr: |
          histogram_quantile(
            0.95,
            sum by (le) (
              rate(forge_http_request_duration_seconds_bucket{
                job="forge",route=~"/api/v1/.*"
              }[5m])
            )
          ) > 2
        for: 10m

      - alert: ForgeRequestSaturation
        expr: forge_http_requests_in_flight{job="forge"} > 200
        for: 5m

      - alert: ForgeJudgeQueueSaturated
        expr: |
          forge_judge_jobs_queued{job="forge"}
          /
          clamp_min(forge_judge_queue_capacity{job="forge"}, 1)
          > 0.8
        for: 5m

      - alert: ForgeJudgeHandoffFailures
        expr: increase(forge_judge_enqueue_failures_total{job="forge"}[10m]) > 20
        for: 10m
```

## Runbook

1. Check `GET /healthz`; a failure means the process is not accepting work.
2. Check `GET /readyz`; it verifies the SQLite write path, data directory,
   test-data directory, free disk, and enabled judge dependencies.
3. Correlate the failing request using `X-Trace-Id` and the structured access
   log, then inspect the preceding `ERROR` entry with the same `traceId`.
4. For elevated 5xx rates, compare the `route` and `status` labels before
   restarting. Preserve logs and the SQLite WAL files before rollback.
5. For high latency, separate SSE routes from the API. A long-lived SSE
   request is expected to remain in flight; short JSON API latency should stay
   below the alert threshold.
6. If readiness fails because of disk space, stop writes, capture the backup
   using `scripts/backup.sh`, free space, and verify `PRAGMA integrity_check`
   before restarting.
7. If `forge_judge_jobs_queued` remains high while workers are healthy, check
   `forge_judge_enqueue_failures_total` and worker logs. Queued submissions are
   durable and the reconciler retries them automatically; do not resubmit from
   a student account to force a new attempt.

## External blocker

Automatic judging remains disabled in production until submissions are
executed by an isolated sandbox worker. Metrics and readiness checks do not
change that security boundary.

