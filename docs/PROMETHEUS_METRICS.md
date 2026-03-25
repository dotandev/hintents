# Prometheus Metrics for Remote Node Health

This document describes the Prometheus metrics exported by the ERST daemon for monitoring remote node health and enabling staleness alerting.

## Overview

The ERST daemon exposes Prometheus metrics at the `/metrics` endpoint when running in daemon mode. These metrics track the health and performance of remote Stellar nodes (Horizon and Soroban RPC endpoints) used during simulation operations.

## Accessing Metrics

Start the daemon with:
```bash
erst daemon --port 8080 --network testnet
```

Metrics are available at:
```
http://localhost:8080/metrics
```

## Available Metrics

### 1. `remote_node_last_response_timestamp_seconds`

**Type:** Gauge  
**Description:** Unix timestamp (in seconds) of the last successful simulation response from a remote node.

**Labels:**
- `node_address`: The RPC URL or identifier of the remote node (e.g., `https://soroban-testnet.stellar.org`)
- `network`: The Stellar network (`testnet`, `mainnet`, `futurenet`)

**Purpose:** This metric enables staleness alerting by tracking when each remote node last successfully responded. The timestamp is only updated on successful responses, so if a node stops responding, the timestamp will remain stale.

**Example PromQL Queries:**

Alert when no response received in 60 seconds:
```promql
time() - remote_node_last_response_timestamp_seconds{node_address="https://soroban-testnet.stellar.org"} > 60
```

Alert when any node hasn't responded in 5 minutes:
```promql
time() - remote_node_last_response_timestamp_seconds > 300
```

Alert when testnet nodes are stale:
```promql
time() - remote_node_last_response_timestamp_seconds{network="testnet"} > 120
```

### 2. `remote_node_response_total`

**Type:** Counter  
**Description:** Total number of simulation responses from remote nodes, labeled by status.

**Labels:**
- `node_address`: The RPC URL or identifier of the remote node
- `network`: The Stellar network (`testnet`, `mainnet`, `futurenet`)
- `status`: Response status (`success`, `error`)

**Purpose:** Track overall node health and error rates over time.

**Example PromQL Queries:**

Alert when error rate exceeds 10% over 5 minutes:
```promql
rate(remote_node_response_total{status="error"}[5m]) / rate(remote_node_response_total[5m]) > 0.1
```

Total successful responses per node:
```promql
sum by (node_address) (remote_node_response_total{status="success"})
```

Error rate by network:
```promql
sum by (network) (rate(remote_node_response_total{status="error"}[5m]))
```

### 3. `remote_node_response_duration_seconds`

**Type:** Histogram  
**Description:** Duration of simulation requests to remote nodes in seconds.

**Labels:**
- `node_address`: The RPC URL or identifier of the remote node
- `network`: The Stellar network (`testnet`, `mainnet`, `futurenet`)

**Buckets:** `[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]`

**Purpose:** Identify performance degradation or latency issues with remote nodes.

**Example PromQL Queries:**

Alert when p95 latency exceeds 5 seconds:
```promql
histogram_quantile(0.95, rate(remote_node_response_duration_seconds_bucket[5m])) > 5
```

Average response time per node:
```promql
rate(remote_node_response_duration_seconds_sum[5m]) / rate(remote_node_response_duration_seconds_count[5m])
```

p99 latency by network:
```promql
histogram_quantile(0.99, sum by (network, le) (rate(remote_node_response_duration_seconds_bucket[5m])))
```

### 4. `simulation_execution_total`

**Type:** Counter  
**Description:** Total number of simulation executions, regardless of remote node involvement.

**Labels:**
- `status`: Execution status (`success`, `error`)

**Purpose:** Track overall system throughput and simulation success rate.

**Example PromQL Queries:**

Alert when simulation error rate exceeds 5%:
```promql
rate(simulation_execution_total{status="error"}[5m]) / rate(simulation_execution_total[5m]) > 0.05
```

Total simulations per minute:
```promql
rate(simulation_execution_total[1m]) * 60
```

## Prometheus Configuration

Add the ERST daemon as a scrape target in your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'erst-daemon'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Alerting Rules

Example Prometheus alerting rules for remote node health:

```yaml
groups:
  - name: erst_remote_node_health
    interval: 30s
    rules:
      # Alert when a node hasn't responded in 60 seconds
      - alert: RemoteNodeStale
        expr: time() - remote_node_last_response_timestamp_seconds > 60
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Remote node {{ $labels.node_address }} is stale"
          description: "Node {{ $labels.node_address }} on {{ $labels.network }} hasn't responded successfully in {{ $value }} seconds"

      # Alert when a node hasn't responded in 5 minutes (critical)
      - alert: RemoteNodeDown
        expr: time() - remote_node_last_response_timestamp_seconds > 300
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Remote node {{ $labels.node_address }} appears down"
          description: "Node {{ $labels.node_address }} on {{ $labels.network }} hasn't responded successfully in {{ $value }} seconds"

      # Alert when error rate is high
      - alert: RemoteNodeHighErrorRate
        expr: |
          rate(remote_node_response_total{status="error"}[5m]) 
          / 
          rate(remote_node_response_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate for {{ $labels.node_address }}"
          description: "Node {{ $labels.node_address }} has {{ $value | humanizePercentage }} error rate"

      # Alert when latency is high
      - alert: RemoteNodeHighLatency
        expr: |
          histogram_quantile(0.95, 
            rate(remote_node_response_duration_seconds_bucket[5m])
          ) > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency for {{ $labels.node_address }}"
          description: "Node {{ $labels.node_address }} p95 latency is {{ $value }}s"

      # Alert when overall simulation error rate is high
      - alert: SimulationHighErrorRate
        expr: |
          rate(simulation_execution_total{status="error"}[5m]) 
          / 
          rate(simulation_execution_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High simulation error rate"
          description: "Simulation error rate is {{ $value | humanizePercentage }}"
```

## Grafana Dashboard

Example Grafana dashboard panels:

### Node Staleness Panel
```json
{
  "title": "Time Since Last Successful Response",
  "targets": [
    {
      "expr": "time() - remote_node_last_response_timestamp_seconds",
      "legendFormat": "{{ node_address }}"
    }
  ],
  "yAxis": {
    "label": "Seconds"
  }
}
```

### Error Rate Panel
```json
{
  "title": "Remote Node Error Rate",
  "targets": [
    {
      "expr": "rate(remote_node_response_total{status=\"error\"}[5m]) / rate(remote_node_response_total[5m])",
      "legendFormat": "{{ node_address }}"
    }
  ],
  "yAxis": {
    "label": "Error Rate",
    "format": "percentunit"
  }
}
```

### Latency Panel
```json
{
  "title": "Remote Node Response Latency (p95)",
  "targets": [
    {
      "expr": "histogram_quantile(0.95, rate(remote_node_response_duration_seconds_bucket[5m]))",
      "legendFormat": "{{ node_address }}"
    }
  ],
  "yAxis": {
    "label": "Seconds"
  }
}
```

## Testing Metrics

### Manual Verification

1. Start the daemon:
   ```bash
   erst daemon --port 8080 --network testnet
   ```

2. Trigger some simulations (via RPC calls or CLI commands)

3. Check metrics:
   ```bash
   curl http://localhost:8080/metrics | grep remote_node
   ```

4. Verify the timestamp updates on successful responses:
   ```bash
   # Run multiple times and observe timestamp changes
   curl http://localhost:8080/metrics | grep remote_node_last_response_timestamp_seconds
   ```

5. Test staleness detection by stopping simulations and observing that the timestamp remains constant while `time() - timestamp` increases.

### Expected Metric Output

```
# HELP remote_node_last_response_timestamp_seconds Unix timestamp of the last successful simulation response from a remote node
# TYPE remote_node_last_response_timestamp_seconds gauge
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 1.709123456e+09
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://soroban-testnet.stellar.org"} 1.709123457e+09

# HELP remote_node_response_duration_seconds Duration of simulation requests to remote nodes in seconds
# TYPE remote_node_response_duration_seconds histogram
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.005"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.01"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.025"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.05"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.1"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.25"} 5
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="0.5"} 10
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="1"} 15
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="2.5"} 18
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="5"} 20
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="10"} 20
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://soroban-testnet.stellar.org",le="+Inf"} 20
remote_node_response_duration_seconds_sum{network="testnet",node_address="https://soroban-testnet.stellar.org"} 8.5
remote_node_response_duration_seconds_count{network="testnet",node_address="https://soroban-testnet.stellar.org"} 20

# HELP remote_node_response_total Total number of simulation responses from remote nodes by status
# TYPE remote_node_response_total counter
remote_node_response_total{network="testnet",node_address="https://soroban-testnet.stellar.org",status="success"} 18
remote_node_response_total{network="testnet",node_address="https://soroban-testnet.stellar.org",status="error"} 2

# HELP simulation_execution_total Total number of simulation executions by status
# TYPE simulation_execution_total counter
simulation_execution_total{status="success"} 45
simulation_execution_total{status="error"} 3
```

## Implementation Details

The metrics are automatically recorded at the following points:

1. **Remote Node Responses**: Metrics are recorded in `internal/rpc/client.go` for:
   - `GetTransaction` calls to Horizon
   - `GetLedgerEntries` calls to Soroban RPC
   - Other RPC methods that interact with remote nodes

2. **Simulation Executions**: Metrics are recorded in `internal/simulator/runner.go` for every simulation run.

3. **Timestamp Updates**: The `remote_node_last_response_timestamp_seconds` gauge is only updated on successful responses, ensuring it accurately reflects the last time the node was healthy.

## Troubleshooting

### Metrics not appearing

1. Verify the daemon is running: `curl http://localhost:8080/health`
2. Check the metrics endpoint: `curl http://localhost:8080/metrics`
3. Ensure simulations are being executed (metrics won't appear until first use)

### Timestamp not updating

1. Verify simulations are succeeding (check logs)
2. Confirm the node is responding successfully
3. Check for errors in the daemon logs

### High error rates

1. Check network connectivity to remote nodes
2. Verify the remote node URLs are correct
3. Check if the remote nodes are experiencing issues
4. Review daemon logs for specific error messages
