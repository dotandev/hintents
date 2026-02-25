/**
 * HSM Signing Load Test - Benchmarking enterprise-scale transaction signing
 * Issue #342: Benchmark the exact performance capacity of audit:sign when dealing with 10k transactions
 *
 * This load test verifies the performance of the audit:sign signing pipeline
 * under sustained high-volume transaction signing scenarios. It measures:
 * - Throughput: transactions signed per second
 * - Latency: p50, p95, p99 latencies for individual signing operations
 * - Resource consumption: memory and CPU utilization during peak load
 * - Error rates: transaction rejection/failure rates
 *
 * Success Criteria:
 * - Minimum 100 signatures per second (allows 10k txns in ~100 seconds)
 * - p95 latency < 100ms per signature
 * - p99 latency < 250ms per signature
 * - Memory growth < 500MB for 10k transactions
 * - Zero failed signatures (0% error rate)
 */

import { performance } from 'perf_hooks';
import { randomBytes } from 'crypto';
import { SoftwareEd25519Signer } from '../src/audit/signing/softwareSigner';
import { AuditLogger } from '../src/audit/AuditLogger';

interface ExecutionTrace {
  input: Record<string, any>;
  state: Record<string, any>;
  events: any[];
  timestamp: string; // ISO string
}

interface LoadTestConfig {
  transactionCount: number;
  payloadSizeBytes: number;
  batchSize: number;
  warmupTransactions: number;
  verbose: boolean;
}

interface SigningMetrics {
  timestamp: number;
  latencyMs: number;
  success: boolean;
  payloadSize: number;
}

interface LoadTestResults {
  totalTransactions: number;
  successfulTransactions: number;
  failedTransactions: number;
  totalTimeMs: number;
  throughputTxnsPerSec: number;
  latencyMetrics: {
    minMs: number;
    maxMs: number;
    meanMs: number;
    p50Ms: number;
    p95Ms: number;
    p99Ms: number;
  };
  memoryMetrics: {
    initialHeapMB: number;
    peakHeapMB: number;
    finalHeapMB: number;
    heapGrowthMB: number;
  };
  errorRate: number;
}

/**
 * Generate a sample transaction trace with realistic structure
 */
function generateMockTrace(transactionId: string, index: number): ExecutionTrace {
  return {
    input: {
      address: `GAPW5GVJ3LHZC7XCYMQGLC2XHFAPVXAHXNLNQWJ34BVUMFG75SMZULH`,
      amount: Math.floor(Math.random() * 1000000),
      asset: 'native',
    },
    state: {
      balanceBefore: Math.floor(Math.random() * 10000000),
      balanceAfter: Math.floor(Math.random() * 10000000),
      nonce: Math.floor(Math.random() * 1000000),
    },
    events: [
      {
        id: randomBytes(16).toString('hex'),
        type: 'contract_call',
        contract: 'CCQY2K5GU5TRANPC2GMYFRBEE74EPSMQTHVWAPZ2SHLU4YPPSTKCPLT',
        method: 'transfer',
        duration_nanos: Math.floor(Math.random() * 1000000),
      },
      {
        id: randomBytes(16).toString('hex'),
        type: 'state_write',
        key: randomBytes(32).toString('hex'),
        bytes_written: Math.floor(Math.random() * 10000),
      },
    ],
    timestamp: new Date().toISOString(),
  };
}

/**
 * Perform a single signing operation and record metrics
 */
async function signTransaction(
  auditLogger: AuditLogger,
  trace: ExecutionTrace,
  index: number
): Promise<SigningMetrics> {
  const startTime = performance.now();
  const payloadSize = JSON.stringify(trace).length;

  try {
    await auditLogger.generateLog(trace);
    const endTime = performance.now();
    const latencyMs = endTime - startTime;

    return {
      timestamp: startTime,
      latencyMs,
      success: true,
      payloadSize,
    };
  } catch (error) {
    const endTime = performance.now();
    const latencyMs = endTime - startTime;

    return {
      timestamp: startTime,
      latencyMs,
      success: false,
      payloadSize,
    };
  }
}

/**
 * Calculate percentile from sorted array
 */
function calculatePercentile(sortedArray: number[], percentile: number): number {
  const index = Math.ceil((percentile / 100) * sortedArray.length) - 1;
  return sortedArray[Math.max(0, index)];
}

/**
 * Run the full load test with warmup, sustained load, and cooldown phases
 */
export async function runLoadTest(config: Partial<LoadTestConfig> = {}): Promise<LoadTestResults> {
  const fullConfig: LoadTestConfig = {
    transactionCount: 10000,
    payloadSizeBytes: 1024,
    batchSize: 100,
    warmupTransactions: 50,
    verbose: false,
    ...config,
  };

  if (fullConfig.verbose) {
    console.log(`
╔════════════════════════════════════════════════════════════════╗
║  HSM Signing Load Test - Enterprise Scale Performance Analysis  ║
╚════════════════════════════════════════════════════════════════╝

Configuration:
  • Total Transactions: ${fullConfig.transactionCount.toLocaleString()}
  • Batch Size: ${fullConfig.batchSize.toLocaleString()}
  • Warmup Transactions: ${fullConfig.warmupTransactions.toLocaleString()}
  • Payload Size: ${fullConfig.payloadSizeBytes} bytes

Test Phases:
  1. Warmup (${fullConfig.warmupTransactions} txns): Initialize system state
  2. Sustained Load (${fullConfig.transactionCount} txns): Main benchmark
  3. Metrics Analysis: Calculate percentiles and statistics

Running benchmark...
    `);
  }

  // Load test Ed25519 private key from environment variable
  // SECURITY: Never hardcode secrets. Use environment variables instead.
  // Generate test key with:
  //   npx ts-node -e "import {generateKeyPairSync} from 'crypto'; const {privateKey} = generateKeyPairSync('ed25519'); console.log(privateKey.export({type: 'pkcs8', format: 'pem'}))"
  // Then set: export ERST_AUDIT_PRIVATE_KEY_PEM='<generated-key>'
  const testPrivateKeyPem = process.env.ERST_AUDIT_PRIVATE_KEY_PEM || (() => {
    throw new Error(
      'ERST_AUDIT_PRIVATE_KEY_PEM environment variable not set. ' +
      'Generate a test key with: npx ts-node -e "import {generateKeyPairSync} from \'crypto\'; const {privateKey} = generateKeyPairSync(\'ed25519\'); console.log(privateKey.export({type: \'pkcs8\', format: \'pem\'}))"'
    );
  })();

  const signer = new SoftwareEd25519Signer(testPrivateKeyPem);
  const auditLogger = new AuditLogger(signer, 'software');

  const metrics: SigningMetrics[] = [];
  let memoryPeak = 0;

  // Measure initial memory state
  const initialMemory = process.memoryUsage();
  const initialHeapMB = initialMemory.heapUsed / 1024 / 1024;

  // Phase 1: Warmup
  if (fullConfig.verbose) console.log(`Phase 1: Warmup (${fullConfig.warmupTransactions} transactions)`);
  for (let i = 0; i < fullConfig.warmupTransactions; i++) {
    const trace = generateMockTrace(`warmup-${i}`, i);
    await signTransaction(auditLogger, trace, i);
  }

  // Phase 2: Sustained Load
  if (fullConfig.verbose) console.log(`Phase 2: Sustained Load (${fullConfig.transactionCount} transactions)`);

  const loadTestStartTime = performance.now();

  for (let i = 0; i < fullConfig.transactionCount; i++) {
    const trace = generateMockTrace(`load-${i}`, fullConfig.warmupTransactions + i);
    const metric = await signTransaction(auditLogger, trace, fullConfig.warmupTransactions + i);
    metrics.push(metric);

    // Monitor memory growth
    if (i % 1000 === 0) {
      const currentMemory = process.memoryUsage();
      const currentHeapMB = currentMemory.heapUsed / 1024 / 1024;
      memoryPeak = Math.max(memoryPeak, currentHeapMB);

      if (fullConfig.verbose && i > 0) {
        const elapsed = performance.now() - loadTestStartTime;
        const throughput = (i / elapsed) * 1000;
        console.log(
          `  Progress: ${i.toLocaleString()}/${fullConfig.transactionCount.toLocaleString()} txns ` +
            `(${throughput.toFixed(1)} txns/sec) - Heap: ${currentHeapMB.toFixed(1)}MB`
        );
      }
    }

    // Optional: Process in smaller batches to allow garbage collection
    if (i % fullConfig.batchSize === 0) {
      await new Promise((resolve) => setTimeout(resolve, 0));
    }
  }

  const loadTestEndTime = performance.now();
  const totalTimeMs = loadTestEndTime - loadTestStartTime;

  // Collect final memory state
  const finalMemory = process.memoryUsage();
  const finalHeapMB = finalMemory.heapUsed / 1024 / 1024;
  const peakHeapMB = Math.max(memoryPeak, finalHeapMB);

  // Calculate statistics
  const successfulTransactions = metrics.filter((m) => m.success).length;
  const failedTransactions = metrics.length - successfulTransactions;
  const errorRate = (failedTransactions / metrics.length) * 100;

  const latencies = metrics.map((m) => m.latencyMs).sort((a, b) => a - b);
  const meanLatency = latencies.reduce((a, b) => a + b, 0) / latencies.length;

  const results: LoadTestResults = {
    totalTransactions: fullConfig.transactionCount,
    successfulTransactions,
    failedTransactions,
    totalTimeMs,
    throughputTxnsPerSec: (fullConfig.transactionCount / totalTimeMs) * 1000,
    latencyMetrics: {
      minMs: Math.min(...latencies),
      maxMs: Math.max(...latencies),
      meanMs: meanLatency,
      p50Ms: calculatePercentile(latencies, 50),
      p95Ms: calculatePercentile(latencies, 95),
      p99Ms: calculatePercentile(latencies, 99),
    },
    memoryMetrics: {
      initialHeapMB: initialHeapMB,
      peakHeapMB,
      finalHeapMB,
      heapGrowthMB: peakHeapMB - initialHeapMB,
    },
    errorRate,
  };

  return results;
}

/**
 * Pretty-print load test results
 */
export function printLoadTestResults(results: LoadTestResults): void {
  console.log(`
╔════════════════════════════════════════════════════════════════╗
║                    LOAD TEST RESULTS SUMMARY                   ║
╚════════════════════════════════════════════════════════════════╝

TRANSACTION RESULTS
  Total Transactions:     ${results.totalTransactions.toLocaleString()}
  Successful:            ${results.successfulTransactions.toLocaleString()} (${((results.successfulTransactions / results.totalTransactions) * 100).toFixed(2)}%)
  Failed:                ${results.failedTransactions.toLocaleString()} (${results.errorRate.toFixed(2)}%)

THROUGHPUT & TIMING
  Total Time:            ${results.totalTimeMs.toFixed(2)}ms (${(results.totalTimeMs / 1000).toFixed(2)}s)
  Throughput:            ${results.throughputTxnsPerSec.toFixed(2)} txns/sec
  ↳ Enterprise Target:   100+ txns/sec [OK]${results.throughputTxnsPerSec >= 100 ? ' PASS' : ' FAIL'}

LATENCY ANALYSIS
  Min Latency:           ${results.latencyMetrics.minMs.toFixed(2)}ms
  P50 Latency:           ${results.latencyMetrics.p50Ms.toFixed(2)}ms
  P95 Latency:           ${results.latencyMetrics.p95Ms.toFixed(2)}ms (target: <100ms) [OK]${results.latencyMetrics.p95Ms < 100 ? ' PASS' : ' FAIL'}
  P99 Latency:           ${results.latencyMetrics.p99Ms.toFixed(2)}ms (target: <250ms) [OK]${results.latencyMetrics.p99Ms < 250 ? ' PASS' : ' FAIL'}
  Max Latency:           ${results.latencyMetrics.maxMs.toFixed(2)}ms
  Mean Latency:          ${results.latencyMetrics.meanMs.toFixed(2)}ms

MEMORY CONSUMPTION
  Initial Heap:          ${results.memoryMetrics.initialHeapMB.toFixed(2)}MB
  Peak Heap:             ${results.memoryMetrics.peakHeapMB.toFixed(2)}MB
  Final Heap:            ${results.memoryMetrics.finalHeapMB.toFixed(2)}MB
  Growth:                ${results.memoryMetrics.heapGrowthMB.toFixed(2)}MB (target: <500MB) [OK]${results.memoryMetrics.heapGrowthMB < 500 ? ' PASS' : ' FAIL'}

ENTERPRISE SCALE READINESS
  10k Transaction Load:  [OK]${results.successfulTransactions === results.totalTransactions ? ' 100% PASS' : ' PARTIAL PASS'}
  Error-free Signing:    [OK]${results.errorRate === 0 ? ' PASS' : ' FAIL'} (${results.errorRate.toFixed(2)}% error rate)
  Latency SLO Met:       [OK]${results.latencyMetrics.p95Ms < 100 && results.latencyMetrics.p99Ms < 250 ? ' PASS' : ' FAIL'}
  Throughput SLO Met:    [OK]${results.throughputTxnsPerSec >= 100 ? ' PASS' : ' FAIL'}
  Memory Budget Ok:      [OK]${results.memoryMetrics.heapGrowthMB < 500 ? ' PASS' : ' FAIL'}

SUMMARY
  Status: ${
    results.throughputTxnsPerSec >= 100 &&
    results.latencyMetrics.p95Ms < 100 &&
    results.latencyMetrics.p99Ms < 250 &&
    results.memoryMetrics.heapGrowthMB < 500 &&
    results.errorRate === 0
      ? '[OK] ENTERPRISE READY'
      : '⚠ REQUIRES OPTIMIZATION'
  }
  
════════════════════════════════════════════════════════════════
  `);
}

/**
 * Main entry point for CLI execution
 */
if (require.main === module) {
  (async () => {
    try {
      const results = await runLoadTest({ verbose: true, transactionCount: 10000 });
      printLoadTestResults(results);
    } catch (error) {
      console.error('Load test failed:', error);
      process.exit(1);
    }
  })();
}

export { LoadTestConfig, LoadTestResults, SigningMetrics };
