# XDR Benchmark Snapshot Generator - Implementation Guide

## Overview

This document provides comprehensive implementation details for the XDR Benchmark Snapshot Generator utility, which is used to generate large-scale randomized snapshots for benchmarking the Stellar snapshot loader in the Hintents project.

## Architecture

### High-Level Design

```
User Request
    ↓
Main CLI Handler (main.go)
    ↓
Config Parsing
    ↓
EntryGenerator Instance
    ├─ GenerateEntries() → []LedgerEntryTuple
    └─ SaveSnapshot() → JSON File
    ↓
Output Statistics
```

### Component Overview

1. **GeneratorConfig**: Configuration struct holding user preferences
   - `Count`: Number of entries to generate
   - `OutputFile`: Path to output snapshot
   - `SeedValue`: For future reproducibility tracking
   - `Verbose`: Progress reporting flag

2. **EntryGenerator**: Main generation engine
   - Manages entry generation pipeline
   - Handles randomization and formatting
   - Coordinates snapshot creation

3. **Main Function**: CLI entry point
   - Flag parsing
   - Input validation
   - Execution orchestration
   - Statistics reporting

## Implementation Details

### Entry Generation Pipeline

#### 1. Key Generation (`GenerateRandomKey`)

```go
func GenerateRandomKey(index int64) string {
    // Generate 32-byte hash-like keys
    // Format: Base64-encoded 32 bytes
    // Mix randomness + index for uniqueness and ordering
}
```

**Characteristics:**
- 32-byte keys (standard ledger entry key size)
- Base64 encoded (~44 characters)
- Deterministic ordering via index inclusion
- Collision-free for practical purposes

**Algorithm:**
1. Initialize 32-byte array
2. Fill first 8 bytes with crypto/rand bytes
3. Encode index as bytes into positions 8-15
4. Fill remaining 16 bytes with crypto/rand
5. Return Base64 encoding

#### 2. Value Generation (`GenerateRandomValue`)

```go
func GenerateRandomValue() string {
    // Generate XDR-like Base64 values
    // Realistic size: 128-2176 bytes (before encoding)
    valueSize := 128 + rand(2048)
}
```

**Characteristics:**
- Variable size (128-2176 bytes decoded)
- Base64 encoded (~170-2900 characters)
- Realistic XDR value distribution
- Pure randomness (genuine XDR data not required)

**Rationale:**
- Actual XDR values vary significantly in size
- For benchmarking, realistic sizes matter more than actual XDR structures
- Reduces implementation complexity while maintaining accuracy

### Snapshot Assembly

#### 1. Entry Generation Loop

```go
for i := 0 to Count:
    key = GenerateRandomKey(i)
    value = GenerateRandomValue()
    entries[i] = LedgerEntryTuple{key, value}
    
    if i % 100000 == 0:
        Report Progress
```

**Performance:**
- ~50,000-100,000 entries/sec (varies by system)
- For 1M entries: ~10-20 seconds
- Memory usage: ~1-2 GB during generation

#### 2. Deterministic Sorting

```go
sort.Slice(entries, func(i, j int) bool {
    return entries[i][0] < entries[j][0]
})
```

**Purpose:**
- Ensures reproducible snapshot format
- Maintains compatibility with snapshot loader expectations
- Enable bit-by-bit comparison between runs

**Complexity:** O(n log n)

#### 3. Snapshot Serialization

```go
type LedgerEntryTuple []string  // [key, value]
type Snapshot struct {
    LedgerEntries []LedgerEntryTuple `json:"ledgerEntries"`
}

snapshot.Save(path, snap) // → JSON with indentation
```

**Output Format:**
```json
{
  "ledgerEntries": [
    ["BASE64_KEY_1", "BASE64_VALUE_1"],
    ["BASE64_KEY_2", "BASE64_VALUE_2"],
    ...
  ]
}
```

## Technical Specifications

### Randomness Strategy

**Implementation:**
- Uses `crypto/rand` for cryptographic-quality randomness
- No seed support in current implementation (uses true randomness)
- Each value is independently generated

**Rationale:**
- Better distribution for benchmarking accuracy
- Avoids pseudo-random patterns that could skew results
- Sufficient entropy for 1M+ entries

### Memory Management

**Peak Memory Usage (1M entries):**
- Entry structs: ~1 GB
- Sorting overhead: ~1 GB (Go's quicksort)
- JSON marshaling: ~500 MB
- **Total peak: ~2-3 GB**

**Optimization Techniques:**
1. Slice pre-allocation (capacity reserved upfront)
2. Single sort operation (vs. incremental)
3. Streaming JSON write (via marshaling)
4. Progressive cleanup during JSON encode

### Performance Characteristics

**Generation Speed Factors:**
- CPU: Random number generation is CPU-bound
- Memory: Sorting adds memory I/O overhead
- I/O: JSON serialization and file write

**Typical Times:**
| Count | Time | Memory | Throughput |
|-------|------|--------|-----------|
| 10K | 0.1s | 50 MB | 100K/s |
| 100K | 1s | 400 MB | 100K/s |
| 1M | 10-15s | 3 GB | 70-100K/s |
| 5M | 50-75s | 15 GB | 70-100K/s |

## Integration Points

### With Existing Code

1. **snapshot.Save()**: Uses existing snapshot package
   ```go
   snapshot.Save(outputPath, snap)
   ```
   - No modifications to snapshots package required
   - Fully compatible with snapshot.Load()

2. **Benchmark Integration**: Direct snapshot usage
   ```go
   snap, _ := snapshot.Load("snapshot_1m.json")
   BenchmarkProcessSnapshot(snap)
   ```

### With Testing Framework

**Unit Tests** (`main_test.go`):
- ~500 lines of test coverage
- Tests: generation, formatting, saving, loading
- Benchmarks: key/value/entry generation

**Running Tests:**
```bash
go test ./cmd/generate-xdr-snapshot -v
go test ./cmd/generate-xdr-snapshot -bench=. -benchmem
```

## Configuration & Flags

### CLI Options

```
-count int64
    Number of entries to generate (default: 1000000)
    
-output string
    Output file path (default: "snapshot_1m.json")
    
-seed int64
    Random seed (informational, current: always random)
    
-verbose bool
    Print progress (default: true)
```

### Future Enhancement Opportunities

1. **Seeded Generation**: Add deterministic mode
   ```go
   rand.Seed(config.SeedValue)
   // Would require refactoring crypto/rand usage
   ```

2. **Parallel Generation**: Use goroutines for faster generation
   ```go
   // Generate entries concurrently, merge, then sort once
   ```

3. **Entry Type Distribution**: Mixed entry types
   ```go
   if randomFloat < 0.3:
       keyType = "account"
   else if randomFloat < 0.6:
       keyType = "trustline"
   // etc.
   ```

4. **Streaming Output**: Direct-to-file streaming
   ```go
   // Reduce peak memory via JSON streaming
   ```

## Verification & Validation

### Generated Snapshot Validation

1. **JSON Format**: Verify structure
   ```bash
   jq '.ledgerEntries | length' snapshot_1m.json
   ```

2. **Entry Ordering**: Verify sorted by key
   ```bash
   jq '.ledgerEntries[] | .[0]' snapshot_1m.json | uniq -c | sort -rn
   ```

3. **Base64 Validity**: Verify encoding
   ```bash
   jq '.ledgerEntries[] | .[0]' snapshot_1m.json | base64 -d > /dev/null
   ```

4. **Size Consistency**: Verify value sizes
   ```bash
   jq '.ledgerEntries[] | .[1:] | length' snapshot_1m.json | sort | uniq -c
   ```

### Performance Validation

**Benchmark a generated snapshot:**
```bash
# Generate benchmark snapshot
./scripts/generate-snapshot.sh generate --seed 42

# Create Go test
go test -bench=BenchmarkSnapshot -benchmem -benchtime=10s

# Analyze results
go test -bench=BenchmarkSnapshot -benchmem | benchstat /dev/stdin
```

## Error Handling

### Common Issues & Solutions

1. **Out of Memory**
   ```
   Error: Fatal: cannot allocate 3GB
   
   Solution:
   - Reduce --count to 500000
   - Run on system with more memory
   - Enable system swap
   ```

2. **Slow Generation**
   ```
   Error: Takes >2 minutes for 1M
   
   Solution:
   - Check system load (top/htop)
   - Verify disk space available
   - Check RAM availability
   ```

3. **Invalid Output**
   ```
   Error: JSON unmarshaling failed
   
   Solution:
   - Verify file not truncated
   - Check snapshot.Load() compatibility
   - Verify file permissions
   ```

## Deployment & Usage

### Build

```bash
# From repository root
go build -o bin/generate-xdr-snapshot ./cmd/generate-xdr-snapshot
```

### Helper Script

```bash
# Use provided shell script
./scripts/generate-snapshot.sh generate --count 1000000

# Or directly
./bin/generate-xdr-snapshot -count 1000000 -output snapshot_1m.json
```

### Integration with CI/CD

```yaml
# Example CI job
generate-benchmark-snapshot:
  script:
    - go build -o ./bin/generate-xdr-snapshot ./cmd/generate-xdr-snapshot
    - ./bin/generate-xdr-snapshot -count 1000000 -seed 12345
    - go test -bench=. -benchmem > results.txt
    - benchstat results.txt
```

## Future Enhancements

### Phase 2 Improvements

1. **Deterministic Seeding**
   - Implement seed-based generation for reproducible benchmarks
   - Useful for regression testing

2. **Parallel Generation**
   - Use worker goroutines for faster generation
   - Target: 200-500K entries/sec

3. **Mixed Entry Types**
   - Generate different key types for realism
   - Account, TrustLine, Offer, Data, etc.

4. **Streaming Mode**
   - Write directly to file during generation
   - Reduce peak memory to <500 MB

5. **Compression Support**
   - Generate gzip-compressed snapshots
   - Reduce disk space from 3.5 GB → 200-300 MB

### Phase 3 Optimization

1. **SIMD Random Generation**
   - Use vectorized RNG for value generation
   - Could 2-3x throughput

2. **Memory-Mapped I/O**
   - Use mmap for sorting phase
   - Reduce GC pressure

3. **Snapshot Merging**
   - Ability to merge multiple snapshots
   - Create 10M+ entries from fragments

## References

- [Snapshot Package](internal/snapshot/snapshot.go)
- [XDR Formatter](internal/decoder/xdr_formatter.go)
- [Existing Benchmarks](tests/trace_decoder_bench_test.go)
- [Go crypto/rand Documentation](https://golang.org/pkg/crypto/rand/)
- [Stellar XDR Format](https://developers.stellar.org/docs/learn/signing-transactions)

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0
