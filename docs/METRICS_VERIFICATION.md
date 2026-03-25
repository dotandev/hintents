# Metrics Verification Guide

This guide walks through manual verification of the Prometheus metrics for remote node health monitoring.

## Prerequisites

- ERST daemon running
- Access to a Stellar network (testnet recommended)
- `curl` or similar HTTP client

## Step 1: Start the Daemon

Start the ERST daemon on port 8080:

```bash
erst daemon --port 8080 --network testnet
```

Expected output:
```
INFO Starting JSON-RPC server port=8080
```

## Step 2: Verify Health Endpoint

Check that the daemon is running:

```bash
curl http://localhost:8080/health
```

Expected output:
```json
{"status":"ok"}
```

## Step 3: Check Initial Metrics

Before any simulations, check the metrics endpoint:

```bash
curl http://localhost:8080/metrics
```

You should see Prometheus metric definitions but no data yet:

```
# HELP remote_node_last_response_timestamp_seconds Unix timestamp of the last successful simulation response from a remote node
# TYPE remote_node_last_response_timestamp_seconds gauge
# HELP remote_node_response_duration_seconds Duration of simulation requests to remote nodes in seconds
# TYPE remote_node_response_duration_seconds histogram
# HELP remote_node_response_total Total number of simulation responses from remote nodes by status
# TYPE remote_node_response_total counter
# HELP simulation_execution_total Total number of simulation executions by status
# TYPE simulation_execution_total counter
```

## Step 4: Trigger a Simulation

Use the ERST CLI to trigger a simulation that will fetch data from remote nodes:

```bash
# Example: Debug a transaction (replace with a real transaction hash)
erst debug <transaction_hash> --network testnet
```

Or use the JSON-RPC API:

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "DebugTransaction",
    "params": {
      "hash": "<transaction_hash>"
    },
    "id": 1
  }'
```

## Step 5: Verify Metrics Updated

Check the metrics again:

```bash
curl http://localhost:8080/metrics | grep remote_node
```

You should now see actual metric values:

```
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 1.709123456e+09
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.005"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.01"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.025"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.05"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.1"} 0
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.25"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="0.5"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="1"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="2.5"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="5"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="10"} 1
remote_node_response_duration_seconds_bucket{network="testnet",node_address="https://horizon-testnet.stellar.org/",le="+Inf"} 1
remote_node_response_duration_seconds_sum{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 0.15
remote_node_response_duration_seconds_count{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 1
remote_node_response_total{network="testnet",node_address="https://horizon-testnet.stellar.org/",status="success"} 1
```

## Step 6: Verify Timestamp Updates

Record the current timestamp:

```bash
curl http://localhost:8080/metrics | grep remote_node_last_response_timestamp_seconds
```

Example output:
```
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 1.709123456e+09
```

Trigger another simulation, then check again:

```bash
curl http://localhost:8080/metrics | grep remote_node_last_response_timestamp_seconds
```

The timestamp should have updated to a more recent value:
```
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 1.709123478e+09
```

## Step 7: Test Staleness Detection

Calculate staleness manually:

```bash
# Get current Unix timestamp
CURRENT_TIME=$(date +%s)

# Get the metric timestamp (extract from metrics output)
METRIC_TIME=$(curl -s http://localhost:8080/metrics | grep 'remote_node_last_response_timestamp_seconds{network="testnet"' | grep -v '#' | awk '{print $2}' | cut -d'.' -f1)

# Calculate staleness
STALENESS=$((CURRENT_TIME - METRIC_TIME))

echo "Staleness: ${STALENESS} seconds"
```

If no simulations have run recently, the staleness value will increase over time.

## Step 8: Verify Error Handling

To test error metrics, you can:

1. Stop the remote node (not practical for public nodes)
2. Use an invalid network configuration
3. Trigger a simulation that will fail

Check error counters:

```bash
curl http://localhost:8080/metrics | grep 'remote_node_response_total.*status="error"'
```

## Step 9: Test with Multiple Nodes

If your configuration uses multiple RPC endpoints, trigger simulations and verify that each node is tracked separately:

```bash
curl http://localhost:8080/metrics | grep remote_node_last_response_timestamp_seconds
```

You should see separate entries for each node:
```
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://horizon-testnet.stellar.org/"} 1.709123456e+09
remote_node_last_response_timestamp_seconds{network="testnet",node_address="https://soroban-testnet.stellar.org"} 1.709123457e+09
```

## Step 10: Verify Simulation Metrics

Check overall simulation execution metrics:

```bash
curl http://localhost:8080/metrics | grep simulation_execution_total
```

Expected output:
```
simulation_execution_total{status="success"} 5
simulation_execution_total{status="error"} 0
```

## Automated Verification Script

Save this as `verify_metrics.sh`:

```bash
#!/bin/bash

DAEMON_URL="http://localhost:8080"
METRICS_URL="${DAEMON_URL}/metrics"

echo "=== ERST Metrics Verification ==="
echo

# Check health
echo "1. Checking daemon health..."
HEALTH=$(curl -s "${DAEMON_URL}/health")
if echo "$HEALTH" | grep -q "ok"; then
    echo "[OK] Daemon is healthy"
else
    echo "[FAIL] Daemon is not responding"
    exit 1
fi
echo

# Check metrics endpoint
echo "2. Checking metrics endpoint..."
METRICS=$(curl -s "$METRICS_URL")
if [ -n "$METRICS" ]; then
    echo "[OK] Metrics endpoint is accessible"
else
    echo "[FAIL] Metrics endpoint is not responding"
    exit 1
fi
echo

# Check for metric definitions
echo "3. Checking metric definitions..."
for metric in "remote_node_last_response_timestamp_seconds" "remote_node_response_total" "remote_node_response_duration_seconds" "simulation_execution_total"; do
    if echo "$METRICS" | grep -q "$metric"; then
        echo "[OK] Found metric: $metric"
    else
        echo "[FAIL] Missing metric: $metric"
    fi
done
echo

# Check for actual data
echo "4. Checking for metric data..."
DATA_COUNT=$(echo "$METRICS" | grep -v '^#' | grep 'remote_node' | wc -l)
if [ "$DATA_COUNT" -gt 0 ]; then
    echo "[OK] Found $DATA_COUNT metric data points"
    echo
    echo "Sample metrics:"
    echo "$METRICS" | grep 'remote_node_last_response_timestamp_seconds{' | head -3
else
    echo "⚠ No metric data yet (trigger some simulations first)"
fi
echo

# Calculate staleness
echo "5. Checking staleness..."
CURRENT_TIME=$(date +%s)
TIMESTAMPS=$(echo "$METRICS" | grep 'remote_node_last_response_timestamp_seconds{' | grep -v '#')

if [ -n "$TIMESTAMPS" ]; then
    echo "$TIMESTAMPS" | while read -r line; do
        NODE=$(echo "$line" | grep -o 'node_address="[^"]*"' | cut -d'"' -f2)
        METRIC_TIME=$(echo "$line" | awk '{print $2}' | cut -d'.' -f1)
        STALENESS=$((CURRENT_TIME - METRIC_TIME))
        
        if [ "$STALENESS" -lt 60 ]; then
            echo "[OK] $NODE: ${STALENESS}s (fresh)"
        elif [ "$STALENESS" -lt 300 ]; then
            echo "⚠ $NODE: ${STALENESS}s (getting stale)"
        else
            echo "[FAIL] $NODE: ${STALENESS}s (STALE)"
        fi
    done
else
    echo "⚠ No timestamp data available"
fi
echo

echo "=== Verification Complete ==="
```

Make it executable and run:

```bash
chmod +x verify_metrics.sh
./verify_metrics.sh
```

## Expected Results

After running simulations, you should see:

1. [OK] All metric definitions present
2. [OK] Timestamp metrics updating on successful responses
3. [OK] Timestamp metrics NOT updating on error responses
4. [OK] Counter metrics incrementing correctly
5. [OK] Histogram buckets populating with latency data
6. [OK] Staleness increasing when no new responses occur
7. [OK] Multiple nodes tracked separately

## Troubleshooting

### No metrics data appearing

- Ensure simulations are being triggered
- Check daemon logs for errors
- Verify network connectivity to remote nodes

### Timestamps not updating

- Confirm simulations are succeeding (check logs)
- Verify the remote node is responding
- Check for authentication or network issues

### Staleness not increasing

- Ensure you're waiting between checks
- Verify no background processes are triggering simulations
- Check system clock is correct

## Next Steps

Once metrics are verified:

1. Configure Prometheus to scrape the metrics endpoint
2. Set up alerting rules (see [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md))
3. Create Grafana dashboards for visualization
4. Monitor metrics in production
