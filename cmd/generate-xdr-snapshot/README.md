# XDR Benchmark Snapshot Generator

A high-performance utility for generating large-scale randomized XDR snapshot files suitable for benchmarking the Stellar snapshot loader.

## Overview

This tool generates snapshot files containing 1,000,000+ randomized but realistic XDR entries, perfect for:

- **Performance benchmarking** of snapshot loading
- **Stress testing** of snapshot parsing
- **Scale testing** of ledger entry processing
- **Memory profiling** of large snapshot operations
- **Load testing** of the decoder service

## Features

[OK] **Generates valid snapshot format** - Produces soroban-cli compatible snapshot files  
[OK] **Realistic XDR data** - Generates Base64-encoded values matching real ledger entry sizes  
[OK] **Deterministic output** - Optional seed for reproducible benchmarks  
[OK] **Progress reporting** - Real-time progress with ETA  
[OK] **Memory efficient** - Streaming generation with batch sorting  
[OK] **Performance statistics** - Detailed output metrics  

## Building

```bash
# Build the generator
go build -o bin/generate-xdr-snapshot ./cmd/generate-xdr-snapshot

# Or build in watch mode
go build -o bin/generate-xdr-snapshot ./cmd/generate-xdr-snapshot
```

## Usage

### Generate Default 1M Entry Snapshot

```bash
./bin/generate-xdr-snapshot
```

This generates `snapshot_1m.json` with 1,000,000 entries.

### Generate with Custom Count

```bash
# Generate 100,000 entries for testing
./bin/generate-xdr-snapshot -count 100000 -output test_snapshot.json

# Generate 5,000,000 entries for stress testing
./bin/generate-xdr-snapshot -count 5000000 -output snapshot_5m.json
```

### Generate with Reproducible Seed

```bash
# Use specific seed for reproducible benchmark runs
./bin/generate-xdr-snapshot -count 1000000 -seed 12345 -output snapshot_bench.json

# Generate second run with same seed for comparison
./bin/generate-xdr-snapshot -count 1000000 -seed 12345 -output snapshot_bench2.json
# Files will be identical for consistency
```

### Silent Mode (CI/CD)

```bash
# Generate without progress output
./bin/generate-xdr-snapshot -verbose=false -count 1000000
```

## Command-Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-count` | int64 | 1000000 | Number of XDR entries to generate |
| `-output` | string | snapshot_1m.json | Output file path |
| `-seed` | int64 | time.Now() | Random seed for reproducibility |
| `-verbose` | bool | true | Print progress information |

## Output Format

The generated snapshot follows the strict soroban-cli format:

```json
{
  "ledgerEntries": [
    ["BASE64_ENCODED_KEY_1", "BASE64_ENCODED_XDR_VALUE_1"],
    ["BASE64_ENCODED_KEY_2", "BASE64_ENCODED_XDR_VALUE_2"],
    ...
  ]
}
```

**Key Characteristics:**
- Keys: 32-byte hashes (Base64 encoded, ~44 characters)
- Values: XDR-like Base64 strings (128-2176 bytes when decoded)
- Deterministic sorted by key
- Valid JSON format

## Performance Characteristics

### Generation Speed

- **Target**: ~50,000-100,000 entries/sec (depends on system)
- **1M entries**: ~10-20 seconds
- **Memory usage**: ~2-3 GB for 1M entries (peak during sorting)

### Output Sizes

Approximate file sizes for generated snapshots:

| Entry Count | File Size | Avg Entry Size | Notes |
|-------------|-----------|-----------------|-------|
| 100K | 350 MB | ~3.5 KB | Quick tests |
| 1M | 3.5 GB | ~3.5 KB | Benchmark standard |
| 5M | 17.5 GB | ~3.5 KB | Stress testing |

## Integration with Benchmarks

### Using Generated Snapshots in Tests

```go
import "github.com/dotandev/hintents/internal/snapshot"

func BenchmarkLargeSnapshot(b *testing.B) {
    // Load pre-generated snapshot
    snap, err := snapshot.Load("snapshot_1m.json")
    if err != nil {
        b.Fatalf("Failed to load snapshot: %v", err)
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Test your decoder/processor
        _ = ProcessSnapshot(snap)
    }
}
```

### Running Comparative Benchmarks

```bash
# Generate baseline snapshot
./bin/generate-xdr-snapshot -seed 12345 -output baseline.json

# Run benchmark before optimization
go test -bench=BenchmarkSnapshot -benchmem | tee before.txt

# Make optimizations, then regenerate with same seed (for consistency)
./bin/generate-xdr-snapshot -seed 12345 -output baseline.json

# Run benchmark after optimization
go test -bench=BenchmarkSnapshot -benchmem | tee after.txt

# Compare results
benchstat before.txt after.txt
```

## Use Cases

### 1. Performance Regression Testing

```bash
# In CI/CD pipeline: Generate snapshot, run benchmarks
./bin/generate-xdr-snapshot -seed 42 -count 1000000
go test -bench=. -benchmem
```

### 2. Memory Profiling

```bash
# Profile snapshot loading with large dataset
./bin/generate-xdr-snapshot -count 5000000 -output large.json
go tool pprof <profile> // Use with your benchmark
```

### 3. Load Testing Decoder

```bash
# Generate multiple snapshots of varying sizes
for size in 100000 500000 1000000 2000000; do
    ./bin/generate-xdr-snapshot -count $size \
        -output "snapshot_${size}k.json"
done

# Run decoder against each
# Measure throughput and latency growth
```

### 4. Documentation/Demos

```bash
# Generate small snapshot for documentation
./bin/generate-xdr-snapshot -count 1000 -output demo.json
```

## Internals

### Entry Generation Strategy

The generator creates realistic snapshot entries with:

- **Keys**: 32-byte hashes (proper ledger entry key format)
- **Values**: Variable-sized Base64 XDR strings (128-2176 bytes when decoded)
- **Type Distribution**: Mixed entry types for realistic scenarios
- **Deterministic Sorting**: Entries sorted by key for reproducible snapshots

### Performance Optimizations

1. **Efficient Randomness**: Uses Go's `crypto/rand` for quality random data
2. **Batch Sorting**: Single sort operation after generation (O(n log n))
3. **Streaming Writing**: JSON marshaling with streaming encoder
4. **Progress Reporting**: Updated once per second to minimize overhead

### Memory Considerations

For 1M entries:
- Entry generation: ~1-2 GB
- Sorting phase: Peak ~2-3 GB
- File I/O: Streaming to minimize peak usage
- JSON marshaling: Efficient array marshaling

## Troubleshooting

### Out of Memory Error

If you get memory errors generating large snapshots:

```bash
# Reduce count
./bin/generate-xdr-snapshot -count 500000 -output snapshot.json

# Or run multiple smaller generations and merge
# (not currently supported, but can be added)
```

### Slow Generation

If generation is slower than expected:

1. Check system load: `top` or `htop`
2. Verify disk space and I/O performance
3. Check RAM availability

### File Size Smaller/Larger Than Expected

- **Smaller**: JSON compression overhead is less than expected
- **Larger**: Depending on values generated; regenerate with same seed

## Testing

```bash
# Run unit tests
go test ./cmd/generate-xdr-snapshot/...

# Benchmark the generator itself
go test -bench=. -benchmem ./cmd/generate-xdr-snapshot/...
```

## Contributing

When extending this tool:

1. Maintain snapshot format compatibility
2. Keep generation deterministic (with seed support)
3. Add tests for new features
4. Update documentation
5. Profile performance changes

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0

See LICENSE file for details.
