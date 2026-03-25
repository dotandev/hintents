# Metrics Testing Guide

This guide covers testing the Prometheus metrics implementation for remote node health monitoring.

## Test Categories

### 1. Unit Tests

Unit tests verify individual metric recording functions work correctly.

**Run unit tests:**
```bash
go test ./internal/metrics -v
```

**Expected output:**
```
=== RUN   TestRecordRemoteNodeResponse_Success
--- PASS: TestRecordRemoteNodeResponse_Success (0.00s)
=== RUN   TestRecordRemoteNodeResponse_Error
--- PASS: TestRecordRemoteNodeResponse_Error (0.00s)
=== RUN   TestRecordRemoteNodeResponse_MultipleNodes
--- PASS: TestRecordRemoteNodeResponse_MultipleNodes (0.00s)
=== RUN   TestRecordSimulationExecution
--- PASS: TestRecordSimulationExecution (0.00s)
=== RUN   TestMetricsLabels
--- PASS: TestMetricsLabels (0.00s)
PASS
ok      github.com/dotandev/hintents/internal/metrics   0.123s
```

**What's tested:**
- [OK] Successful response recording updates timestamp
- [OK] Error response recording does NOT update timestamp
- [OK] Multiple nodes tracked independently
- [OK] Counters increment correctly
- [OK] Histograms record duration data
- [OK] Metric labels are correct

### 2. Integration Tests

Integration tests verify metrics work end-to-end via HTTP.

**Run integration tests:**
```bash
go test -tags=integration ./internal/metrics -v
```

**Expected output:**
```
=== RUN   TestMetricsEndpoint
--- PASS: TestMetricsEndpoint (0.01s)
=== RUN   TestMetricsStalenessDetection
--- PASS: TestMetricsStalenessDetection (2.01s)
=== RUN   TestMetricsMultipleNodes
--- PASS: TestMetricsMultipleNodes (0.01s)
PASS
ok      github.com/dotandev/hintents/internal/metrics   2.234s
```

**What's tested:**
- [OK] Metrics exposed via HTTP endpoint
- [OK] Prometheus format is correct
- [OK] Staleness detection works (timestamp doesn't update on errors)
- [OK] Multiple nodes tracked separately in HTTP output
- [OK] Metric values match expected counts

### 3. Manual Testing

Manual testing verifies the complete system works in a real environment.

#### Setup

1. **Start the daemon:**
   ```bash
   erst daemon --port 8080 --network testnet
   ```

2. **Verify health:**
   ```bash
   curl http://localhost:8080/health
   # Expected: {"status":"ok"}
   ```

3. **Check initial metrics:**
   ```bash
   curl http://localhost:8080/metrics | grep remote_node
   # Expected: Metric definitions but no data yet
   ```

#### Test Case 1: Successful Response Updates Timestamp

**Steps:**
1. Record initial timestamp:
   ```bash
   BEFORE=$(curl -s http://localhost:8080/metrics | grep 'remote_node_last_response_timestamp_seconds{' | head -1 | awk '{print $2}')
   echo "Before: $BEFORE"
   ```

2. Trigger a simulation (replace with real transaction hash):
   ```bash
   erst debug <transaction_hash> --network testnet
   ```

3. Check timestamp updated:
   ```bash
   AFTER=$(curl -s http://localhost:8080/metrics | grep 'remote_node_last_response_timestamp_seconds{' | head -1 | awk '{print $2}')
   echo "After: $AFTER"
   ```

**Expected result:** `AFTER` > `BEFORE`

#### Test Case 2: Error Response Does NOT Update Timestamp

**Steps:**
1. Record timestamp after successful response
2. Trigger a simulation that will fail (invalid hash, network issue, etc.)
3. Check timestamp remains unchanged

**Expected result:** Timestamp stays the same, error counter increments

#### Test Case 3: Staleness Detection

**Steps:**
1. Trigger a successful simulation
2. Wait 60+ seconds without triggering more simulations
3. Calculate staleness:
   ```bash
   CURRENT=$(date +%s)
   METRIC=$(curl -s http://localhost:8080/metrics | grep 'remote_node_last_response_timestamp_seconds{' | head -1 | awk '{print $2}' | cut -d'.' -f1)
   STALENESS=$((CURRENT - METRIC))
   echo "Staleness: ${STALENESS} seconds"
   ```

**Expected result:** Staleness > 60 seconds

#### Test Case 4: Multiple Nodes Tracked Separately

**Steps:**
1. Trigger simulations that use different nodes (Horizon and Soroban RPC)
2. Check metrics:
   ```bash
   curl http://localhost:8080/metrics | grep remote_node_last_response_timestamp_seconds
   ```

**Expected result:** Separate entries for each node address

#### Test Case 5: Histogram Buckets

**Steps:**
1. Trigger multiple simulations
2. Check histogram data:
   ```bash
   curl http://localhost:8080/metrics | grep remote_node_response_duration_seconds_bucket
   ```

**Expected result:** Counts distributed across buckets based on actual latencies

#### Test Case 6: Counter Increments

**Steps:**
1. Note current counter values:
   ```bash
   curl http://localhost:8080/metrics | grep 'remote_node_response_total{.*status="success"}'
   ```

2. Trigger 5 successful simulations
3. Check counters increased by 5

**Expected result:** Counters increment correctly

### 4. Load Testing

Test metrics under load to verify performance.

**Simple load test:**
```bash
#!/bin/bash
# load_test.sh

DAEMON_URL="http://localhost:8080"
ITERATIONS=100

echo "Running $ITERATIONS simulations..."

for i in $(seq 1 $ITERATIONS); do
    # Trigger simulation (adjust command as needed)
    erst debug <transaction_hash> --network testnet &
    
    # Limit concurrent requests
    if [ $((i % 10)) -eq 0 ]; then
        wait
        echo "Completed $i/$ITERATIONS"
    fi
done

wait
echo "Load test complete"

# Check metrics
echo "Checking metrics..."
curl -s "$DAEMON_URL/metrics" | grep simulation_execution_total
```

**Expected result:**
- All simulations complete successfully
- Metrics accurately reflect all executions
- No memory leaks or performance degradation
- Metrics endpoint remains responsive

### 5. Prometheus Integration Testing

Test with actual Prometheus instance.

#### Setup Prometheus

1. **Create `prometheus.yml`:**
   ```yaml
   global:
     scrape_interval: 15s
   
   scrape_configs:
     - job_name: 'erst-daemon'
       static_configs:
         - targets: ['localhost:8080']
       metrics_path: '/metrics'
   ```

2. **Start Prometheus:**
   ```bash
   docker run -p 9090:9090 -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml prom/prometheus
   ```

3. **Access Prometheus UI:**
   ```
   http://localhost:9090
   ```

#### Test Queries

1. **Check target is up:**
   - Navigate to Status > Targets
   - Verify `erst-daemon` target is UP

2. **Query metrics:**
   ```promql
   remote_node_last_response_timestamp_seconds
   ```
   - Should show data for all active nodes

3. **Test staleness query:**
   ```promql
   time() - remote_node_last_response_timestamp_seconds
   ```
   - Should show seconds since last response

4. **Test error rate:**
   ```promql
   rate(remote_node_response_total{status="error"}[5m]) / rate(remote_node_response_total[5m])
   ```
   - Should show error rate as decimal (0.0 to 1.0)

5. **Test latency:**
   ```promql
   histogram_quantile(0.95, rate(remote_node_response_duration_seconds_bucket[5m]))
   ```
   - Should show p95 latency in seconds

### 6. Alert Testing

Test alerting rules work correctly.

#### Setup Alert Rules

1. **Create `alerts.yml`:**
   ```yaml
   groups:
     - name: erst_test
       interval: 10s
       rules:
         - alert: RemoteNodeStale
           expr: time() - remote_node_last_response_timestamp_seconds > 30
           for: 30s
           labels:
             severity: warning
   ```

2. **Update `prometheus.yml`:**
   ```yaml
   rule_files:
     - 'alerts.yml'
   ```

3. **Restart Prometheus**

#### Test Alert Firing

1. Trigger successful simulation
2. Wait 60+ seconds without triggering more
3. Check Prometheus UI > Alerts
4. Verify `RemoteNodeStale` alert fires

**Expected result:** Alert transitions from Inactive → Pending → Firing

### 7. Grafana Integration Testing

Test metrics display correctly in Grafana.

#### Setup Grafana

1. **Start Grafana:**
   ```bash
   docker run -p 3000:3000 grafana/grafana
   ```

2. **Add Prometheus data source:**
   - URL: `http://localhost:9090`
   - Access: Browser

3. **Create test dashboard**

#### Test Panels

1. **Staleness panel:**
   - Query: `time() - remote_node_last_response_timestamp_seconds`
   - Visualization: Time series
   - Expected: Line showing increasing staleness when no responses

2. **Error rate panel:**
   - Query: `rate(remote_node_response_total{status="error"}[5m]) / rate(remote_node_response_total[5m])`
   - Visualization: Time series
   - Expected: Line showing error rate percentage

3. **Latency panel:**
   - Query: `histogram_quantile(0.95, rate(remote_node_response_duration_seconds_bucket[5m]))`
   - Visualization: Time series
   - Expected: Line showing p95 latency

### 8. Regression Testing

Ensure metrics don't break existing functionality.

**Test checklist:**
- [OK] Daemon starts successfully with metrics enabled
- [OK] Simulations run normally with metrics recording
- [OK] No performance degradation in simulation execution
- [OK] No memory leaks from metric recording
- [OK] Existing endpoints (/health, /rpc) still work
- [OK] Logs don't show metric-related errors

### 9. Error Handling Testing

Test metrics handle errors gracefully.

**Test scenarios:**
1. **Prometheus endpoint unavailable:** Metrics still record internally
2. **Invalid metric labels:** Validation prevents invalid labels
3. **Concurrent metric updates:** Thread-safe metric recording
4. **High cardinality:** Metrics don't explode with too many labels

### 10. Documentation Testing

Verify documentation is accurate.

**Test checklist:**
- [OK] All example queries work in Prometheus
- [OK] Alert rules fire as expected
- [OK] Grafana panels display correctly
- [OK] Verification scripts run successfully
- [OK] Code examples compile and run

## Automated Test Suite

Run all tests:
```bash
#!/bin/bash
# run_all_tests.sh

echo "=== Running Unit Tests ==="
go test ./internal/metrics -v

echo ""
echo "=== Running Integration Tests ==="
go test -tags=integration ./internal/metrics -v

echo ""
echo "=== Running Manual Verification ==="
./verify_metrics.sh

echo ""
echo "=== All Tests Complete ==="
```

## Continuous Integration

Add to CI pipeline:
```yaml
# .github/workflows/test.yml
- name: Test Metrics
  run: |
    go test ./internal/metrics -v
    go test -tags=integration ./internal/metrics -v
```

## Test Coverage

Check test coverage:
```bash
go test ./internal/metrics -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Target coverage:** > 80%

## Troubleshooting Tests

### Tests fail with "connection refused"
- Ensure daemon is running on correct port
- Check firewall settings

### Metrics not appearing in tests
- Verify metrics are being recorded (check logs)
- Ensure test triggers actual simulations

### Integration tests timeout
- Increase test timeout
- Check network connectivity

### Prometheus can't scrape metrics
- Verify daemon is accessible from Prometheus container
- Check network configuration (use host network mode if needed)

## Success Criteria

All tests pass when:
- [OK] Unit tests pass (100%)
- [OK] Integration tests pass (100%)
- [OK] Manual verification succeeds
- [OK] Prometheus successfully scrapes metrics
- [OK] Alerts fire correctly
- [OK] Grafana displays metrics
- [OK] No performance degradation
- [OK] No memory leaks
- [OK] Documentation is accurate
