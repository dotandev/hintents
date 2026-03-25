# Prometheus Metrics Quick Reference

Quick reference for DevOps engineers setting up monitoring and alerting for ERST remote node health.

## Metrics Endpoint

```
http://localhost:8080/metrics
```

## Key Metrics

| Metric | Type | Purpose | Alert Threshold |
|--------|------|---------|-----------------|
| `remote_node_last_response_timestamp_seconds` | Gauge | Last successful response time | `time() - metric > 60` |
| `remote_node_response_total` | Counter | Total responses by status | Error rate > 10% |
| `remote_node_response_duration_seconds` | Histogram | Request duration | p95 > 5s |
| `simulation_execution_total` | Counter | Total simulations | Error rate > 5% |

## Common Labels

- `node_address`: RPC URL (e.g., `https://soroban-testnet.stellar.org`)
- `network`: Stellar network (`testnet`, `mainnet`, `futurenet`)
- `status`: Response status (`success`, `error`)

## Essential Alerts

### 1. Node Staleness (Warning)
```yaml
alert: RemoteNodeStale
expr: time() - remote_node_last_response_timestamp_seconds > 60
for: 1m
```

### 2. Node Down (Critical)
```yaml
alert: RemoteNodeDown
expr: time() - remote_node_last_response_timestamp_seconds > 300
for: 2m
```

### 3. High Error Rate
```yaml
alert: RemoteNodeHighErrorRate
expr: |
  rate(remote_node_response_total{status="error"}[5m]) 
  / rate(remote_node_response_total[5m]) > 0.1
for: 5m
```

### 4. High Latency
```yaml
alert: RemoteNodeHighLatency
expr: |
  histogram_quantile(0.95, 
    rate(remote_node_response_duration_seconds_bucket[5m])
  ) > 5
for: 5m
```

## Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: 'erst-daemon'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Quick Queries

### Check node staleness
```promql
time() - remote_node_last_response_timestamp_seconds
```

### Error rate per node
```promql
rate(remote_node_response_total{status="error"}[5m]) 
/ rate(remote_node_response_total[5m])
```

### Average latency
```promql
rate(remote_node_response_duration_seconds_sum[5m]) 
/ rate(remote_node_response_duration_seconds_count[5m])
```

### p95 latency
```promql
histogram_quantile(0.95, 
  rate(remote_node_response_duration_seconds_bucket[5m])
)
```

### Requests per second
```promql
rate(remote_node_response_total[1m])
```

## Verification Commands

### Check daemon health
```bash
curl http://localhost:8080/health
```

### View all metrics
```bash
curl http://localhost:8080/metrics
```

### Check specific node
```bash
curl http://localhost:8080/metrics | grep 'node_address="https://soroban-testnet.stellar.org"'
```

### Calculate staleness
```bash
CURRENT=$(date +%s)
METRIC=$(curl -s http://localhost:8080/metrics | grep remote_node_last_response_timestamp_seconds | awk '{print $2}' | cut -d'.' -f1)
echo "Staleness: $((CURRENT - METRIC)) seconds"
```

## Grafana Panels

### Staleness Panel
```
Query: time() - remote_node_last_response_timestamp_seconds
Legend: {{ node_address }}
Unit: seconds
```

### Error Rate Panel
```
Query: rate(remote_node_response_total{status="error"}[5m]) / rate(remote_node_response_total[5m])
Legend: {{ node_address }}
Unit: percentunit
```

### Latency Panel
```
Query: histogram_quantile(0.95, rate(remote_node_response_duration_seconds_bucket[5m]))
Legend: {{ node_address }} (p95)
Unit: seconds
```

## Troubleshooting

| Issue | Check | Solution |
|-------|-------|----------|
| No metrics | `curl http://localhost:8080/metrics` | Start daemon, trigger simulations |
| Timestamp not updating | Check logs | Verify node connectivity |
| High staleness | Check network | Investigate node health |
| High error rate | Check logs | Review node configuration |

## Documentation Links

- Full guide: [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md)
- Verification: [METRICS_VERIFICATION.md](METRICS_VERIFICATION.md)
- Package docs: [internal/metrics/README.md](../internal/metrics/README.md)
