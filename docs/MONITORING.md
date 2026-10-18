# Monitoring EveChat

## Endpoints

| Endpoint | Auth | Purpose |
|---|---|---|
| `/api/v1/health` | public | liveness + version/commit/uptime/db/hub snapshot |
| `/metrics` | `metrics.token` or loopback | Prometheus exposition |
| `/api/v1/diagnostics` | admin+ | full doctor report as JSON |
| `evechat doctor --json` | local | same report from the CLI |

Enable metrics:

```yaml
metrics:
  enabled: true
  token: "a-long-random-string"
```

Scrape with `Authorization: Bearer <token>`; without a token the endpoint only
answers from `127.0.0.1`/`::1`.

## Metrics worth alerting on

| Metric | Meaning |
|---|---|
| `evechat_http_requests_total{status}` | error spikes (5xx) |
| `evechat_http_request_duration_seconds` | p95 latency |
| `evechat_ws_connections` | connected widget/agent sockets |
| `evechat_ws_clients_dropped_total` | slow consumers evicted |
| `evechat_messages_total` | throughput |
| `evechat_rate_limited_total` | abuse or tight limits |
| `evechat_goroutines`, `evechat_memory_bytes{kind}` | leaks |
| `evechat_conversations_total{status}` | backlog |
| `evechat_build_info` | deployed version |

Included files: `deploy/monitoring/prometheus.yml`, `alerts.yml` and a Grafana
dashboard (`grafana-dashboard.json`).

## Golden signals checklist

1. **Availability:** `up == 0` for 2m → page.
2. **Latency:** p95 > 1s for 10m → investigate disk/CPU.
3. **Errors:** 5xx rate > 1% → check `journalctl -u evechat`.
4. **Saturation:** memory > 200MB or goroutines growing monotonically.

## Logs

- stdout JSON (systemd/journald) or `log.file` with internal size rotation
  (`log.max_size_mb`, `log.max_backups`).
- Every request carries `request_id`; handlers log with the same id, and the
  response header `X-Request-Id` lets users report a traceable id.
- Duplicate lines are sampled (max 1/sec) to keep disk usage bounded.

## Health probes

```bash
curl -fsS localhost:3000/api/v1/health | jq '{status,version,setup_complete,db}'
evechat healthcheck --config /opt/evechat/evechat.yaml
```

Docker healthcheck uses `evechat healthcheck`; systemd relies on `Restart=always`
plus the `/api/v1/health` endpoint (wire it to your external monitor).
