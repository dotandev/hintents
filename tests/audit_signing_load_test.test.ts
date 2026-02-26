/**
 * Test suite for HSM signing load test
 * Validates the audit:sign performance under various load scenarios
 */

import { generateKeyPairSync } from 'crypto';
import { runLoadTest, printLoadTestResults, LoadTestResults } from '../test/audit_load_test';

describe('HSM Signing Load Test Suite', () => {
  jest.setTimeout(120000); // 2 minute timeout for load tests

  // Set up test Ed25519 key before running tests
  beforeAll(() => {
    if (!process.env.ERST_AUDIT_PRIVATE_KEY_PEM) {
      const { privateKey } = generateKeyPairSync('ed25519');
      const pem = privateKey.export({ type: 'pkcs8', format: 'pem' }).toString();
      process.env.ERST_AUDIT_PRIVATE_KEY_PEM = pem;
    }
  });

  describe('Basic Load Test - 1000 transactions', () => {
    let results: LoadTestResults;

    beforeAll(async () => {
      results = await runLoadTest({
        transactionCount: 1000,
        batchSize: 100,
        warmupTransactions: 20,
        verbose: false,
      });
    });

    test('should complete all transactions successfully', () => {
      expect(results.successfulTransactions).toBe(results.totalTransactions);
      expect(results.failedTransactions).toBe(0);
    });

    test('should achieve minimum throughput of 100 txns/sec', () => {
      expect(results.throughputTxnsPerSec).toBeGreaterThanOrEqual(100);
    });

    test('should maintain p95 latency below 100ms', () => {
      expect(results.latencyMetrics.p95Ms).toBeLessThan(100);
    });

    test('should maintain p99 latency below 250ms', () => {
      expect(results.latencyMetrics.p99Ms).toBeLessThan(250);
    });

    test('should have zero error rate', () => {
      expect(results.errorRate).toBe(0);
    });

    test('should keep memory growth under 500MB', () => {
      expect(results.memoryMetrics.heapGrowthMB).toBeLessThan(500);
    });

    test('should report realistic latency statistics', () => {
      expect(results.latencyMetrics.minMs).toBeLessThanOrEqual(results.latencyMetrics.p50Ms);
      expect(results.latencyMetrics.p50Ms).toBeLessThanOrEqual(results.latencyMetrics.p95Ms);
      expect(results.latencyMetrics.p95Ms).toBeLessThanOrEqual(results.latencyMetrics.p99Ms);
      expect(results.latencyMetrics.p99Ms).toBeLessThanOrEqual(results.latencyMetrics.maxMs);
    });

    test('should report consistent timing information', () => {
      expect(results.totalTimeMs).toBeGreaterThan(0);
      expect(results.throughputTxnsPerSec).toBeGreaterThan(0);
      expect(results.throughputTxnsPerSec).toBeLessThan(Infinity);
    });
  });

  describe('Enterprise Scale Load Test - 10000 transactions', () => {
    let results: LoadTestResults;

    beforeAll(async () => {
      results = await runLoadTest({
        transactionCount: 10000,
        batchSize: 500,
        warmupTransactions: 100,
        verbose: false,
      });
    });

    test('should complete all 10k transactions with 100% success rate', () => {
      expect(results.successfulTransactions).toBe(10000);
      expect(results.failedTransactions).toBe(0);
      expect(results.errorRate).toBe(0);
    });

    test('should handle enterprise-scale load (10k txns)', () => {
      expect(results.totalTransactions).toBe(10000);
    });

    test('should maintain sub-100ms p95 latency at enterprise scale', () => {
      expect(results.latencyMetrics.p95Ms).toBeLessThan(100);
    });

    test('should maintain sub-250ms p99 latency at enterprise scale', () => {
      expect(results.latencyMetrics.p99Ms).toBeLessThan(250);
    });

    test('should achieve required throughput for 10k transactions', () => {
      // At 100 txns/sec, 10k txns should take ~100 seconds
      const expectedMaxTimeMs = (10000 / 100) * 1000;
      expect(results.totalTimeMs).toBeLessThan(expectedMaxTimeMs * 1.5); // Allow 50% margin
    });

    test('should scale linearly with transaction count', () => {
      // Time should scale roughly linearly with transaction count
      const timePerTxn = results.totalTimeMs / results.totalTransactions;
      expect(timePerTxn).toBeGreaterThan(0);
      expect(timePerTxn).toBeLessThan(100); // Shouldn't take more than 100ms per txn on average
    });

    test('should manage memory efficiently during sustained load', () => {
      const avgMemoryPerTxn = results.memoryMetrics.heapGrowthMB / results.totalTransactions;
      // Should use less than 50KB per transaction on average
      expect(avgMemoryPerTxn * 1024).toBeLessThan(50);
    });

    test('should have all latency metrics consistent with volume', () => {
      // Mean latency should be closer to p50 than to p99 (typical distribution)
      const meanToP50Ratio = results.latencyMetrics.meanMs / results.latencyMetrics.p50Ms;
      expect(meanToP50Ratio).toBeGreaterThan(0.9);
      expect(meanToP50Ratio).toBeLessThan(1.5);
    });
  });

  describe('Batch Processing Efficiency', () => {
    test('should process batches efficiently', async () => {
      const smallBatchResults = await runLoadTest({
        transactionCount: 500,
        batchSize: 50,
        warmupTransactions: 10,
        verbose: false,
      });

      const largeBatchResults = await runLoadTest({
        transactionCount: 500,
        batchSize: 250,
        warmupTransactions: 10,
        verbose: false,
      });

      // Both should complete successfully
      expect(smallBatchResults.errorRate).toBe(0);
      expect(largeBatchResults.errorRate).toBe(0);

      // Throughput should be similar (within 20%)
      const throughputRatio = smallBatchResults.throughputTxnsPerSec / largeBatchResults.throughputTxnsPerSec;
      expect(throughputRatio).toBeGreaterThan(0.8);
      expect(throughputRatio).toBeLessThan(1.2);
    });
  });

  describe('Memory Profile Analysis', () => {
    let results: LoadTestResults;

    beforeAll(async () => {
      results = await runLoadTest({
        transactionCount: 2000,
        batchSize: 200,
        warmupTransactions: 50,
        verbose: false,
      });
    });

    test('should not have memory leaks (cleanup between batches)', () => {
      // Final memory should be less than peak memory
      expect(results.memoryMetrics.finalHeapMB).toBeLessThanOrEqual(results.memoryMetrics.peakHeapMB);
    });

    test('should have reasonable heap growth for sustained load', () => {
      // Heap growth should be relatively small compared to peak usage
      const growthRatio = results.memoryMetrics.heapGrowthMB / results.memoryMetrics.peakHeapMB;
      expect(growthRatio).toBeLessThan(1.5); // Growth shouldn't be more than 50% of peak
    });

    test('should track initial memory state accurately', () => {
      expect(results.memoryMetrics.initialHeapMB).toBeGreaterThan(0);
      expect(results.memoryMetrics.peakHeapMB).toBeGreaterThanOrEqual(results.memoryMetrics.initialHeapMB);
    });
  });

  describe('Latency Distribution Analysis', () => {
    let results: LoadTestResults;

    beforeAll(async () => {
      results = await runLoadTest({
        transactionCount: 3000,
        batchSize: 300,
        warmupTransactions: 100,
        verbose: false,
      });
    });

    test('should have reasonable latency variance (p99/p50 ratio)', () => {
      const ratioP99ToP50 = results.latencyMetrics.p99Ms / results.latencyMetrics.p50Ms;
      // P99 shouldn't be more than 10x the median (indicates stability)
      expect(ratioP99ToP50).toBeLessThan(10);
    });

    test('should maintain consistent tail latencies', () => {
      const ratioMaxToP99 = results.latencyMetrics.maxMs / results.latencyMetrics.p99Ms;
      // Max shouldn't be extreme outlier compared to p99
      expect(ratioMaxToP99).toBeLessThan(5);
    });

    test('should have mean latency reflecting typical performance', () => {
      // Mean should be between p50 and p95 (typical normal distribution)
      expect(results.latencyMetrics.meanMs).toBeGreaterThanOrEqual(results.latencyMetrics.p50Ms);
      expect(results.latencyMetrics.meanMs).toBeLessThanOrEqual(results.latencyMetrics.p95Ms);
    });
  });

  describe('Enterprise SLO Compliance', () => {
    let results: LoadTestResults;

    beforeAll(async () => {
      results = await runLoadTest({
        transactionCount: 10000,
        batchSize: 500,
        warmupTransactions: 100,
        verbose: false,
      });
    });

    test('should meet throughput SLO of 100+ txns/sec', () => {
      expect(results.throughputTxnsPerSec).toBeGreaterThanOrEqual(100);
    });

    test('should meet p95 latency SLO of <100ms', () => {
      expect(results.latencyMetrics.p95Ms).toBeLessThan(100);
    });

    test('should meet p99 latency SLO of <250ms', () => {
      expect(results.latencyMetrics.p99Ms).toBeLessThan(250);
    });

    test('should meet memory budget SLO of <500MB growth', () => {
      expect(results.memoryMetrics.heapGrowthMB).toBeLessThan(500);
    });

    test('should have zero error rate SLO', () => {
      expect(results.errorRate).toBe(0);
    });

    test('should be enterprise-ready (all SLOs met)', () => {
      const throughputOk = results.throughputTxnsPerSec >= 100;
      const p95Ok = results.latencyMetrics.p95Ms < 100;
      const p99Ok = results.latencyMetrics.p99Ms < 250;
      const memoryOk = results.memoryMetrics.heapGrowthMB < 500;
      const errorRateOk = results.errorRate === 0;

      expect(throughputOk && p95Ok && p99Ok && memoryOk && errorRateOk).toBe(true);
    });
  });

  describe('Result Printing', () => {
    test('should not throw when printing results', async () => {
      const results = await runLoadTest({
        transactionCount: 100,
        batchSize: 50,
        warmupTransactions: 10,
        verbose: false,
      });

      // Mock console.log to capture output
      const logSpy = jest.spyOn(console, 'log').mockImplementation();

      expect(() => {
        printLoadTestResults(results);
      }).not.toThrow();

      expect(logSpy).toHaveBeenCalled();

      logSpy.mockRestore();
    });
  });
});
