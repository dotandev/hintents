# Coverage-Guided Simulator Fuzzer

The Coverage-Guided Simulator Fuzzer is a critical tool for finding edge cases, potential panics, and security vulnerabilities in Soroban smart contracts before deployment. It uses coverage feedback to intelligently guide the generation of new test inputs.

## Overview

This package implements a sophisticated fuzzing engine that:

- **Maximizes Code Coverage**: Uses coverage feedback from the simulator to generate inputs that exercise new code paths
- **Intelligent Mutation**: Applies multiple mutation strategies (bitflip, byteflip, interesting values, havoc)
- **Crash Detection**: Automatically detects and records inputs that cause contract failures
- **Corpus Management**: Maintains and evolves a corpus of interesting test cases
- **Performance Optimized**: Efficient execution with configurable timeouts and throughput tracking

## Architecture

### Components

1. **CoverageGuidedFuzzer**: Main fuzzer engine that orchestrates the fuzzing campaign
2. **FuzzerConfig**: Configuration for fuzzing behavior and parameters
3. **MutationStrategy**: Different input mutation techniques
4. **CorpusEntry**: Individual test cases with associated coverage information
5. **CoverageMap**: Tracks coverage feedback from each execution

### Mutation Strategies

- **Bitflip**: Flips individual bits in the input (useful for finding boundary conditions)
- **Byteflip**: Replaces entire bytes with random values (explores value space)
- **Interesting**: Uses known interesting byte patterns (0x00, 0x7f, 0x80, 0xff, etc.)
- **Havoc**: Applies multiple random mutations (intensive exploration)

## Usage

### Basic Fuzzing

```go
package main

import (
	"context"
	"encoding/hex"
	"log"

	"github.com/dotandev/hintents/internal/fuzz"
	"github.com/dotandev/hintents/internal/simulator"
)

func main() {
	// Create a simulator runner
	runner, err := simulator.NewRunner("", false)
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}
	defer runner.Close()

	// Configure the fuzzer
	config := fuzz.FuzzerConfig{
		MaxIterations:  1000,
		TimeoutMs:      5000,
		MaxCorpusSize:  500,
		EnableCoverage: true,
		VerboseLogging: true,
	}

	// Create fuzzer
	fuzzer := fuzz.NewCoverageGuidedFuzzer(runner, config)

	// Create seed input
	seedInput := &simulator.FuzzerInput{
		EnvelopeXdr: hex.EncodeToString([]byte("test data")),
		Timestamp:   1234567890,
	}

	// Run fuzzing campaign
	ctx := context.Background()
	stats, err := fuzzer.Run(ctx, seedInput)
	if err != nil {
		log.Fatalf("Fuzzing failed: %v", err)
	}

	// Print results
	println("Crashes found:", stats.CrashCount)
	println("Corpus size:", stats.CorpusSize)
	println("Executions/sec:", stats.ExecutionsPerSecond())

	// Retrieve crashing inputs
	crashes := fuzzer.GetCrashingInputs()
	for i, crash := range crashes {
		println("Crash", i+1, ":", crash.EnvelopeXdr)
	}
}
```

### Configuration Options

```go
config := fuzz.FuzzerConfig{
	MaxIterations:      10000,          // Number of fuzzing iterations
	TimeoutMs:          5000,           // Timeout per execution (ms)
	MaxCorpusSize:      1000,           // Max test cases to keep
	CoverageSampleRate: 0.2,            // Probability of recording coverage
	EnableCoverage:     true,           // Enable coverage-guided fuzzing
	TargetContractID:   "CAxxxxx",      // Optional: target specific contract
	Seed:               42,             // RNG seed for reproducibility
	VerboseLogging:     true,           // Enable detailed logging
	MutationStrategies: []fuzz.MutationStrategy{
		fuzz.StrategyBitflip,
		fuzz.StrategyInteresting,
		fuzz.StrategyHavoc,
	},
}
```

### Retrieving Results

```go
// Get statistics from the fuzzing run
stats, _ := fuzzer.Run(ctx, seedInput)
println("Duration:", stats.Duration())
println("Executions:", stats.ExecutionCount)
println("Crashes:", stats.CrashCount)
println("New coverage found:", stats.NewCoverageCount)

// Get crashing inputs for further analysis
crashes := fuzzer.GetCrashingInputs()

// Get the evolved corpus
corpus := fuzzer.GetCorpus()

// Get coverage statistics
covStats := fuzzer.CoverageStats()
println("Unique coverage signatures:", covStats.UniqueCoverageCount)
println("Max coverage reached:", covStats.MaxCoverage)
```

## Performance

The fuzzer is designed for high throughput:

- **Typical throughput**: 50-500+ executions/second (depends on contract complexity)
- **Memory efficient**: Corpus size is bounded by `MaxCorpusSize`
- **Scalable**: Can run for hours with appropriate timeouts
- **Parallelizable**: Can run multiple fuzzer instances with different seeds

Example performance on a simple contract:
```
Fuzzing Complete!
Duration: 10.5s
Total Executions: 5000
Crashes Found: 3
New Coverage Found: 127
Final Corpus Size: 342
Executions/sec: 476.19
```

## Advanced Features

### Custom Seed Initialization

```go
seedInput := &simulator.FuzzerInput{
	EnvelopeXdr: "deadbeef...",
	Timestamp:   time.Now().Unix(),
	LedgerEntries: map[string]string{
		"contract:ABC123": "state_data",
		"account:DEF456":  "balance_data",
	},
	Args: []string{
		hex.EncodeToString([]byte("arg1")),
		hex.EncodeToString([]byte("arg2")),
	},
}
```

### Timeout Handling

The fuzzer respects context timeouts and will gracefully stop when:
- `ctx` is cancelled
- `ctx` deadline is reached
- `MaxIterations` is reached

```go
// Run with 5-minute fuzzing limit
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

stats, _ := fuzzer.Run(ctx, seedInput)
```

### Coverage-Guided Evolution

The fuzzer uses coverage feedback to evolve the corpus:

1. Select entry from corpus
2. Mutate the entry
3. Execute and collect coverage
4. If new coverage found, add to corpus
5. Prioritize futures mutations from high-coverage entries

## Testing and Validation

### Running Tests

```bash
# Run all fuzzer tests
go test ./internal/fuzz/...

# Run specific test
go test -run TestMutateInput ./internal/fuzz/...

# Run with race detection
go test -race ./internal/fuzz/...

# Run benchmarks
go test -bench=Mutation ./internal/fuzz/... -benchmem
```

### Unit Test Coverage

The fuzzer includes comprehensive tests for:
- Fuzzer instantiation
- Configuration defaults
- Mutation strategies
- Corpus management
- Crash tracking
- Coverage statistics
- Context cancellation
- Input execution

### Integration with Simulator

The fuzzer integrates with the simulator runner via the `RunnerInterface`:

```go
// Works with any RunnerInterface implementation
var runner simulator.RunnerInterface

// Can be mocked for testing
mockRunner := simulator.NewMockRunner()
fuzzer := fuzz.NewCoverageGuidedFuzzer(mockRunner, config)
```

## Troubleshooting

### No crashes found

- Increase `MaxIterations` to explore more input space
- Ensure seed input is valid
- Check that contract execution doesn't have unexpected handlers

### Slow execution

- Increase `TimeoutMs` if contracts are genuinely slow
- Reduce corpus size to speed up selection
- Use fewer mutation strategies

### OOM errors

- Reduce `MaxCorpusSize`
- Run multiple smaller fuzzing campaigns
- Enable coverage sampling to reduce memory overhead

## Integration with CI/CD

```bash
# Run fuzzer as part of CI pipeline
./erst fuzz --contract <contract-id> --iterations 10000 --timeout 5000

# Generate test report
./erst fuzz --contract <contract-id> --report coverage.html
```

## References

- [AFL fuzzer](https://lcamtuf.blogspot.com/2014/01/afl-fuzz-making-up-bugs-for-fun-and.html)
- [libFuzzer](https://llvm.org/docs/LibFuzzer/)
- [Cargo Fuzz](https://rust-fuzz.github.io/book/cargo-fuzz.html)

See also:
- `internal/simulator/fuzzing_harness.go` - Lower-level fuzzing harness
- `docs/FLAMEGRAPH_ARCHITECTURE.md` - Performance profiling integration
- `test.md` - Project overview
