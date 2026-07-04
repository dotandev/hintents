// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package fuzz

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/simulator"
)

// CoverageMap represents the code coverage feedback from a single execution
type CoverageMap struct {
	coveredLines  map[string]bool
	totalCoverage uint32
	timestamp     time.Time
}

// CorpusEntry represents a single test case in the fuzzing corpus
type CorpusEntry struct {
	Input       *simulator.FuzzerInput
	Coverage    *CoverageMap
	ResultIdx   int
	NewCoverage bool
	Timestamp   time.Time
}

// CoverageGuidedFuzzer implements a coverage-guided fuzzer for Stellar contracts
type CoverageGuidedFuzzer struct {
	runner           simulator.RunnerInterface
	config           FuzzerConfig
	corpus           []*CorpusEntry
	crashingInputs   []*simulator.FuzzerInput
	coverageMap      map[string]uint32 // Maps coverage signature to count
	seedRng          *rand.Rand
	mu               sync.RWMutex
	executionCount   uint64
	lastCoverageGrow time.Time
}

// FuzzerConfig contains configuration for the coverage-guided fuzzer
type FuzzerConfig struct {
	MaxIterations      uint64
	TimeoutMs          uint64
	MaxCorpusSize      int
	CoverageSampleRate float64 // 0.0-1.0: probability of recording coverage
	MutationStrategies []MutationStrategy
	EnableCoverage     bool
	TargetContractID   string
	Seed               int64
	VerboseLogging     bool
}

// MutationStrategy defines how inputs are mutated
type MutationStrategy string

const (
	// Bitflip mutations flip random bits
	StrategyBitflip MutationStrategy = "bitflip"

	// ByteFlip mutations alter entire bytes
	StrategyByteFlip MutationStrategy = "byteflip"

	// Interesting mutations use interesting byte values
	StrategyInteresting MutationStrategy = "interesting"

	// Dictionary mutations use known keywords
	StrategyDictionary MutationStrategy = "dictionary"

	// Havoc performs random mutations
	StrategyHavoc MutationStrategy = "havoc"
)

// NewCoverageGuidedFuzzer creates a new coverage-guided fuzzer
func NewCoverageGuidedFuzzer(runner simulator.RunnerInterface, config FuzzerConfig) *CoverageGuidedFuzzer {
	if config.MaxIterations == 0 {
		config.MaxIterations = 1000
	}
	if config.TimeoutMs == 0 {
		config.TimeoutMs = 5000
	}
	if config.MaxCorpusSize == 0 {
		config.MaxCorpusSize = 1000
	}
	if config.CoverageSampleRate == 0 {
		config.CoverageSampleRate = 0.1 // 10% default
	}
	if len(config.MutationStrategies) == 0 {
		config.MutationStrategies = []MutationStrategy{
			StrategyBitflip,
			StrategyByteFlip,
			StrategyInteresting,
			StrategyHavoc,
		}
	}

	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &CoverageGuidedFuzzer{
		runner:           runner,
		config:           config,
		corpus:           make([]*CorpusEntry, 0, config.MaxCorpusSize),
		crashingInputs:   make([]*simulator.FuzzerInput, 0),
		coverageMap:      make(map[string]uint32),
		seedRng:          rand.New(rand.NewSource(seed)),
		lastCoverageGrow: time.Now(),
	}
}

// Run executes the coverage-guided fuzzing campaign
func (f *CoverageGuidedFuzzer) Run(ctx context.Context, seedInput *simulator.FuzzerInput) (*FuzzingStats, error) {
	if seedInput == nil {
		return nil, fmt.Errorf("seed input required for fuzzing")
	}

	stats := &FuzzingStats{
		StartTime: time.Now(),
	}

	// Add seed input to corpus
	f.addToCorpus(ctx, seedInput, nil)

	// Main fuzzing loop
	for i := uint64(0); i < f.config.MaxIterations && ctx.Err() == nil; i++ {
		// Select corpus entry for mutation (favor entries with recent coverage gains)
		entry := f.selectCorpusEntry()
		if entry == nil {
			break
		}

		// Mutate the selected input
		mutated := f.mutateInput(entry.Input)

		// Run the simulator
		result, coverage := f.executeInput(ctx, &mutated)

		// Track crashes
		if result.Status == "crash" {
			f.mu.Lock()
			f.crashingInputs = append(f.crashingInputs, &mutated)
			f.mu.Unlock()
			stats.CrashCount++
		}

		// Update corpus if new coverage found
		newCoverage := f.addToCorpus(ctx, &mutated, coverage)
		if newCoverage {
			stats.NewCoverageCount++
			f.lastCoverageGrow = time.Now()
		}

		stats.ExecutionCount = atomic.AddUint64(&f.executionCount, 1)

		// Log progress periodically
		if f.config.VerboseLogging && (i+1)%100 == 0 {
			f.logProgress(i + 1)
		}
	}

	stats.EndTime = time.Now()
	stats.CorpusSize = len(f.corpus)
	stats.CoverageEntryCount = len(f.coverageMap)
	stats.UniqueInputsCount = len(f.corpus)

	return stats, nil
}

// addToCorpus adds an input to the corpus if it improves coverage
// Returns true if the input was added (new coverage found)
func (f *CoverageGuidedFuzzer) addToCorpus(ctx context.Context, input *simulator.FuzzerInput, coverage *CoverageMap) bool {
	if coverage == nil {
		coverage = f.extractCoverage(input)
	}

	if !f.config.EnableCoverage {
		// If coverage tracking disabled, keep at least one corpus entry if empty.
		if len(f.corpus) == 0 && len(f.corpus) < f.config.MaxCorpusSize {
			f.mu.Lock()
			entry := &CorpusEntry{
				Input:       input,
				Timestamp:   time.Now(),
				NewCoverage: false,
			}
			f.corpus = append(f.corpus, entry)
			f.mu.Unlock()
			return false
		}

		if f.seedRng.Float64() < 0.05 && len(f.corpus) < f.config.MaxCorpusSize {
			f.mu.Lock()
			entry := &CorpusEntry{
				Input:       input,
				Timestamp:   time.Now(),
				NewCoverage: false,
			}
			f.corpus = append(f.corpus, entry)
			f.mu.Unlock()
			return false
		}
		return false
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if this coverage signature is new
	sig := coverage.computeSignature()
	if _, exists := f.coverageMap[sig]; !exists {
		// New coverage found - add to corpus if space available
		if len(f.corpus) < f.config.MaxCorpusSize {
			entry := &CorpusEntry{
				Input:       input,
				Coverage:    coverage,
				Timestamp:   time.Now(),
				NewCoverage: true,
			}
			f.corpus = append(f.corpus, entry)
			f.coverageMap[sig] = coverage.totalCoverage

			if f.config.VerboseLogging {
				logger.Logger.Info(
					"New coverage found",
					"corpus_size", len(f.corpus),
					"coverage_id", sig[:8],
				)
			}
			return true
		}
	}

	return false
}

// selectCorpusEntry selects an entry from the corpus favoring recent ones
func (f *CoverageGuidedFuzzer) selectCorpusEntry() *CorpusEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.corpus) == 0 {
		return nil
	}

	// Simple strategy: favor entries with recent new coverage
	// Sort by recency and select from top entries with weighted randomness
	if len(f.corpus) == 1 {
		return f.corpus[0]
	}

	// Pick a random entry, slightly favoring recent ones
	idx := f.seedRng.Intn(len(f.corpus))
	if f.seedRng.Float64() < 0.3 && len(f.corpus) > 1 {
		// 30% chance to pick from the most recent 10%
		recentIdx := f.seedRng.Intn((len(f.corpus) / 10) + 1)
		idx = len(f.corpus) - 1 - recentIdx
	}

	return f.corpus[idx]
}

// mutateInput applies mutation strategies to create a new input
func (f *CoverageGuidedFuzzer) mutateInput(base *simulator.FuzzerInput) simulator.FuzzerInput {
	rng := rand.New(rand.NewSource(f.seedRng.Int63()))

	mutated := simulator.FuzzerInput{
		EnvelopeXdr:   base.EnvelopeXdr,
		Timestamp:     base.Timestamp,
		LedgerEntries: make(map[string]string),
		Args:          make([]string, len(base.Args)),
		Seed:          f.seedRng.Uint64(),
	}

	// Copy inputs
	for k, v := range base.LedgerEntries {
		mutated.LedgerEntries[k] = v
	}
	copy(mutated.Args, base.Args)

	// Select a random mutation strategy
	strategy := f.config.MutationStrategies[rng.Intn(len(f.config.MutationStrategies))]

	// Apply mutations
	switch strategy {
	case StrategyBitflip:
		f.applyBitflipMutation(&mutated, rng)
	case StrategyByteFlip:
		f.applyByteflipMutation(&mutated, rng)
	case StrategyInteresting:
		f.applyInterestingMutation(&mutated, rng)
	case StrategyHavoc:
		f.applyHavocMutation(&mutated, rng)
	default:
		f.applyBitflipMutation(&mutated, rng)
	}

	return mutated
}

// applyBitflipMutation flips random bits in the input
func (f *CoverageGuidedFuzzer) applyBitflipMutation(input *simulator.FuzzerInput, rng *rand.Rand) {
	if len(input.EnvelopeXdr) > 0 {
		data, _ := hex.DecodeString(input.EnvelopeXdr)
		if len(data) > 0 {
			flipCount := 1 + rng.Intn(4)
			for i := 0; i < flipCount; i++ {
				pos := rng.Intn(len(data))
				bit := uint8(1 << (rng.Intn(8)))
				data[pos] ^= bit
			}
			input.EnvelopeXdr = hex.EncodeToString(data)
		}
	}

	// Mutate ledger entries
	for k, v := range input.LedgerEntries {
		if rng.Float64() < 0.5 {
			input.LedgerEntries[k] = f.bitflipHexString(v, rng)
		}
	}
}

// applyByteflipMutation flips entire bytes in the input
func (f *CoverageGuidedFuzzer) applyByteflipMutation(input *simulator.FuzzerInput, rng *rand.Rand) {
	if len(input.EnvelopeXdr) > 0 {
		data, _ := hex.DecodeString(input.EnvelopeXdr)
		if len(data) > 0 {
			flipCount := 1 + rng.Intn(3)
			for i := 0; i < flipCount; i++ {
				pos := rng.Intn(len(data))
				data[pos] = byte(rng.Intn(256))
			}
			input.EnvelopeXdr = hex.EncodeToString(data)
		}
	}
}

// applyInterestingMutation uses known interesting byte values
func (f *CoverageGuidedFuzzer) applyInterestingMutation(input *simulator.FuzzerInput, rng *rand.Rand) {
	interestingBytes := []byte{
		0x00, 0x01, 0x7f, 0x80, 0xff, // Common edge cases
		0x42, 0x43, // Useful for strings
	}

	if len(input.EnvelopeXdr) > 0 {
		data, _ := hex.DecodeString(input.EnvelopeXdr)
		if len(data) > 0 {
			pos := rng.Intn(len(data))
			data[pos] = interestingBytes[rng.Intn(len(interestingBytes))]
			input.EnvelopeXdr = hex.EncodeToString(data)
		}
	}
}

// applyHavocMutation applies random mutations
func (f *CoverageGuidedFuzzer) applyHavocMutation(input *simulator.FuzzerInput, rng *rand.Rand) {
	// Apply 1-5 random mutations
	mutationCount := 1 + rng.Intn(5)
	for i := 0; i < mutationCount; i++ {
		choice := rng.Intn(3)
		switch choice {
		case 0:
			f.applyBitflipMutation(input, rng)
		case 1:
			f.applyByteflipMutation(input, rng)
		case 2:
			f.applyInterestingMutation(input, rng)
		}
	}
}

// bitflipHexString applies bitflip mutations to a hex string
func (f *CoverageGuidedFuzzer) bitflipHexString(hexStr string, rng *rand.Rand) string {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return hexStr
	}

	flipCount := 1 + rng.Intn(3)
	for i := 0; i < flipCount; i++ {
		if len(data) == 0 {
			break
		}
		pos := rng.Intn(len(data))
		bit := uint8(1 << (rng.Intn(8)))
		data[pos] ^= bit
	}

	return hex.EncodeToString(data)
}

// executeInput runs a single input through the simulator
func (f *CoverageGuidedFuzzer) executeInput(ctx context.Context, input *simulator.FuzzerInput) (*simulator.FuzzingResult, *CoverageMap) {
	result := &simulator.FuzzingResult{
		Seed:   input.Seed,
		Status: "pass",
	}

	// Build simulation request
	simReq := &simulator.SimulationRequest{
		EnvelopeXdr:   input.EnvelopeXdr,
		LedgerEntries: input.LedgerEntries,
		Timestamp:     input.Timestamp,
		MockArgs:      &input.Args,
	}

	if f.config.EnableCoverage {
		simReq.EnableCoverage = true
		if simReq.CoverageLCOVPath == nil {
			tmpFile, err := os.CreateTemp("", "erst-fuzz-*.lcov")
			if err != nil {
				result.Status = "error"
				result.ErrorMessage = fmt.Sprintf("failed to create coverage temp file: %v", err)
				result.ExecutionTimeMs = 0
				return result, nil
			}
			coveragePath := tmpFile.Name()
			_ = tmpFile.Close()
			simReq.CoverageLCOVPath = &coveragePath
			defer os.Remove(coveragePath)
		}
	}

	start := time.Now()

	// Run simulation with timeout context
	simResp, err := f.runner.Run(ctx, simReq)
	result.ExecutionTimeMs = uint64(time.Since(start).Milliseconds())
	if err != nil {
		result.Status = "crash"
		result.ErrorMessage = fmt.Sprintf("execution error: %v", err)
		return result, nil
	}

	coverage := f.extractCoverageFromResponse(simResp)
	result.CodeCoverage = coverage.totalCoverage

	// Analyze response
	if simResp.Status == "error" {
		result.Status = "error"
		result.ErrorMessage = simResp.Error
	}

	// Check for slow execution
	if result.ExecutionTimeMs > f.config.TimeoutMs {
		result.Status = "slow"
		result.ErrorMessage = fmt.Sprintf("execution time exceeded %dms", f.config.TimeoutMs)
	}

	return result, coverage
}

// extractCoverage extracts coverage information from an input when explicit coverage is not available.
func (f *CoverageGuidedFuzzer) extractCoverage(input *simulator.FuzzerInput) *CoverageMap {
	coverage := &CoverageMap{
		coveredLines: make(map[string]bool),
		timestamp:    time.Now(),
	}

	hash := f.computeInputHash(input)
	coverage.totalCoverage = uint32(len(hash)) * 8
	coverage.coveredLines[hash] = true

	return coverage
}

// extractCoverageFromResponse parses coverage data returned by the simulator.
func (f *CoverageGuidedFuzzer) extractCoverageFromResponse(resp *simulator.SimulationResponse) *CoverageMap {
	if resp == nil {
		return &CoverageMap{coveredLines: make(map[string]bool), timestamp: time.Now()}
	}

	if resp.LCOVReport != "" {
		return f.parseLCOVReport(resp.LCOVReport)
	}

	if resp.LCOVReportPath != "" {
		content, err := os.ReadFile(resp.LCOVReportPath)
		if err == nil {
			return f.parseLCOVReport(string(content))
		}
	}

	return &CoverageMap{coveredLines: make(map[string]bool), timestamp: time.Now()}
}

func (f *CoverageGuidedFuzzer) parseLCOVReport(report string) *CoverageMap {
	coverage := &CoverageMap{
		coveredLines: make(map[string]bool),
		timestamp:    time.Now(),
	}

	for _, line := range strings.Split(report, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "DA:") {
			continue
		}

		parts := strings.SplitN(line[3:], ",", 2)
		if len(parts) != 2 {
			continue
		}

		lineNum := strings.TrimSpace(parts[0])
		countStr := strings.TrimSpace(parts[1])
		count, err := strconv.Atoi(countStr)
		if err != nil {
			continue
		}

		if count > 0 {
			coverage.coveredLines[lineNum] = true
			coverage.totalCoverage++
		}
	}

	return coverage
}

// computeInputHash creates a simple hash of the input
func (f *CoverageGuidedFuzzer) computeInputHash(input *simulator.FuzzerInput) string {
	return fmt.Sprintf("%s_%d_%d", input.EnvelopeXdr[:min(32, len(input.EnvelopeXdr))],
		input.Timestamp, len(input.LedgerEntries))
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// logProgress logs fuzzing progress
func (f *CoverageGuidedFuzzer) logProgress(iteration uint64) {
	f.mu.RLock()
	corpusSize := len(f.corpus)
	crashCount := len(f.crashingInputs)
	coverageCount := len(f.coverageMap)
	f.mu.RUnlock()

	elapsed := time.Since(f.lastCoverageGrow)
	logger.Logger.Info(
		"Fuzzing progress",
		"iterations", iteration,
		"corpus_size", corpusSize,
		"crashes", crashCount,
		"coverage_entries", coverageCount,
		"time_since_new_coverage", elapsed.String(),
	)
}

// GetCrashingInputs returns all inputs that caused crashes
func (f *CoverageGuidedFuzzer) GetCrashingInputs() []*simulator.FuzzerInput {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*simulator.FuzzerInput, len(f.crashingInputs))
	copy(result, f.crashingInputs)
	return result
}

// GetCorpus returns a copy of the current corpus
func (f *CoverageGuidedFuzzer) GetCorpus() []*CorpusEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*CorpusEntry, len(f.corpus))
	copy(result, f.corpus)
	return result
}

// GetCoverageMap returns coverage statistics
func (f *CoverageGuidedFuzzer) GetCoverageMap() map[string]uint32 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]uint32)
	for k, v := range f.coverageMap {
		result[k] = v
	}
	return result
}

// CoverageStats returns aggregate coverage statistics
func (f *CoverageGuidedFuzzer) CoverageStats() CoverageStatistics {
	f.mu.RLock()
	defer f.mu.RUnlock()

	stats := CoverageStatistics{
		CorpusSize:          len(f.corpus),
		UniqueCoverageCount: len(f.coverageMap),
		CrashCount:          len(f.crashingInputs),
		ExecutionCount:      atomic.LoadUint64(&f.executionCount),
	}

	// Calculate max coverage
	maxCov := uint32(0)
	avgCov := uint32(0)
	totalCov := uint32(0)

	for _, cov := range f.coverageMap {
		if cov > maxCov {
			maxCov = cov
		}
		totalCov += cov
	}

	if len(f.coverageMap) > 0 {
		avgCov = totalCov / uint32(len(f.coverageMap))
	}

	stats.MaxCoverage = maxCov
	stats.AvgCoverage = avgCov
	stats.TimeSinceLastCoverageGrow = time.Since(f.lastCoverageGrow)

	return stats
}

// computeSignature computes a unique signature for coverage
func (cm *CoverageMap) computeSignature() string {
	keys := make([]string, 0, len(cm.coveredLines))
	for k := range cm.coveredLines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sig := ""
	for _, k := range keys {
		sig += k + ","
	}
	return sig
}
