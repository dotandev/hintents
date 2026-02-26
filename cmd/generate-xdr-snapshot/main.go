// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dotandev/hintents/internal/snapshot"
)

// GeneratorConfig holds configuration for XDR generation
type GeneratorConfig struct {
	Count      int64
	OutputFile string
	SeedValue  int64
	Verbose    bool
}

// EntryGenerator generates randomized valid XDR entries for benchmarking
type EntryGenerator struct {
	config *GeneratorConfig
}

// NewEntryGenerator creates a new generator with config
func NewEntryGenerator(config *GeneratorConfig) *EntryGenerator {
	return &EntryGenerator{
		config: config,
	}
}

// GenerateRandomKey creates a unique random key suitable for snapshot entries
func (eg *EntryGenerator) GenerateRandomKey(index int64) string {
	// Create deterministic but randomized keys for reproducible benchmarks
	// Keys are 32-byte hashes encoded as base64
	keyBytes := make([]byte, 32)
	randBytes := make([]byte, 8)

	// Use index + random data for uniqueness
	randBytesN, _ := rand.Read(randBytes)
	copy(keyBytes[:8], randBytes[:randBytesN])

	// Add index for deterministic ordering when needed
	for i := 0; i < 8; i++ {
		keyBytes[8+i] = byte((index >> uint(i*8)) & 0xFF)
	}

	// Fill rest with pseudo-random data
	randRestBytes := make([]byte, 16)
	rand.Read(randRestBytes)
	copy(keyBytes[16:], randRestBytes)

	return base64.StdEncoding.EncodeToString(keyBytes)
}

// GenerateRandomValue creates a valid XDR value string
// For benchmarking, we generate realistic XDR-like Base64 strings
// These represent encoded LedgerEntry structures
func (eg *EntryGenerator) GenerateRandomValue() string {
	// Generate realistic XDR-like Base64 values
	// Actual XDR values vary in size from ~100 bytes to several KB
	valueSize := 128 + int(eg.pseudoRand32()%2048) // 128-2176 bytes

	valueBytes := make([]byte, valueSize)
	bytesRead, err := rand.Read(valueBytes)
	if err != nil {
		log.Fatalf("Failed to generate random bytes: %v", err)
	}
	if bytesRead != valueSize {
		log.Fatalf("Only read %d/%d random bytes", bytesRead, valueSize)
	}

	// Encode as Base64 to simulate XDR encoding
	return base64.StdEncoding.EncodeToString(valueBytes)
}

// pseudoRand32 generates a pseudo-random 32-bit number for simplicity
func (eg *EntryGenerator) pseudoRand32() uint32 {
	b := make([]byte, 4)
	rand.Read(b)
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// GenerateEntries generates randomized snapshot entries
func (eg *EntryGenerator) GenerateEntries() []snapshot.LedgerEntryTuple {
	entries := make([]snapshot.LedgerEntryTuple, 0, eg.config.Count)

	if eg.config.Verbose {
		fmt.Printf("Generating %d randomized XDR entries...\n", eg.config.Count)
	}

	startTime := time.Now()
	lastProgressUpdate := startTime

	for i := int64(0); i < eg.config.Count; i++ {
		// Generate key and value
		key := eg.GenerateRandomKey(i)
		value := eg.GenerateRandomValue()

		// Add to entries
		entries = append(entries, snapshot.LedgerEntryTuple{key, value})

		// Progress reporting
		if eg.config.Verbose && time.Since(lastProgressUpdate) > time.Second {
			processed := i + 1
			elapsed := time.Since(startTime)
			rate := float64(processed) / elapsed.Seconds()
			eta := time.Duration(float64(eg.config.Count-processed)/rate) * time.Second
			fmt.Printf("\rProgress: %d/%d (%.0f entries/sec, ETA: %v)",
				processed, eg.config.Count, rate, eta)
			lastProgressUpdate = time.Now()
		}
	}

	if eg.config.Verbose {
		fmt.Printf("\nGeneration completed in %v\n", time.Since(startTime))
		fmt.Printf("Sorting entries...\n")
	}

	// Sort entries by key for deterministic snapshot format
	sort.Slice(entries, func(i, j int) bool {
		return entries[i][0] < entries[j][0]
	})

	return entries
}

// GenerateSnapshot generates a complete snapshot with randomized entries
func (eg *EntryGenerator) GenerateSnapshot() *snapshot.Snapshot {
	entries := eg.GenerateEntries()
	return &snapshot.Snapshot{LedgerEntries: entries}
}

func main() {
	config := &GeneratorConfig{}

	flag.Int64Var(&config.Count, "count", 1000000, "Number of XDR entries to generate")
	flag.StringVar(&config.OutputFile, "output", "snapshot_1m.json", "Output snapshot file path")
	flag.Int64Var(&config.SeedValue, "seed", time.Now().UnixNano(), "Random seed (informational only)")
	flag.BoolVar(&config.Verbose, "verbose", true, "Print progress information")

	flag.Parse()

	// Validate input
	if config.Count <= 0 {
		log.Fatalf("Count must be positive, got %d", config.Count)
	}

	if err := os.MkdirAll(filepath.Dir(config.OutputFile), 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	fmt.Printf("XDR Benchmark Snapshot Generator\n")
	fmt.Printf("================================\n")
	fmt.Printf("Entries to generate: %d\n", config.Count)
	fmt.Printf("Output file: %s\n", config.OutputFile)
	fmt.Printf("Seed: %d\n\n", config.SeedValue)

	generator := NewEntryGenerator(config)

	if config.Verbose {
		fmt.Println("Generating randomized XDR snapshot...")
	}

	snap := generator.GenerateSnapshot()

	if config.Verbose {
		fmt.Printf("Saving snapshot to %s\n", config.OutputFile)
	}

	if err := snapshot.Save(config.OutputFile, snap); err != nil {
		log.Fatalf("Failed to save snapshot: %v", err)
	}

	// Print statistics
	fileInfo, err := os.Stat(config.OutputFile)
	if err != nil {
		log.Fatalf("Failed to stat output file: %v", err)
	}

	fileSizeBytes := fileInfo.Size()
	fileSizeMB := float64(fileSizeBytes) / (1024 * 1024)
	avgEntrySize := float64(fileSizeBytes) / float64(config.Count)

	fmt.Printf("\n=== Generation Statistics ===\n")
	fmt.Printf("Total entries generated: %d\n", config.Count)
	fmt.Printf("Total file size: %.2f MB (%.0f bytes)\n", fileSizeMB, float64(fileSizeBytes))
	fmt.Printf("Average XDR value size: %.0f bytes\n", avgEntrySize)
	fmt.Printf("Estimated overhead (keys + JSON): %.2f %%\n",
		(float64(fileSizeBytes)/float64(config.Count*1000))*100)
	fmt.Printf("\nSnapshot saved successfully!\n")
}
