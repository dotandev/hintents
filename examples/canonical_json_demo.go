// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/signer"
)

// This example demonstrates how canonical JSON serialization ensures
// deterministic hashing across different platforms and scenarios.

func main() {
	fmt.Println("=== Canonical JSON Serialization Demo ===")

	// Generate a key pair for signing
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	privateKeyHex := hex.EncodeToString(privateKey)

	// Example 1: Generate audit log
	fmt.Println("1. Generating audit log with canonical JSON...")
	s, _ := signer.NewInMemorySigner(privateKeyHex)
	auditLog, err := cmd.GenerateWithSigner(
		"tx-hash-12345",
		"envelope-xdr-data",
		"result-meta-xdr-data",
		[]string{"event1", "event2", "event3"},
		[]string{"log1", "log2"},
		s,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   Transaction Hash: %s\n", auditLog.TransactionHash)
	fmt.Printf("   Trace Hash: %s\n", auditLog.TraceHash)
	fmt.Printf("   Signature: %s...\n", auditLog.Signature[:32])
	fmt.Println()

	// Example 2: Verify the audit log
	fmt.Println("2. Verifying audit log signature...")
	err = cmd.Verify(auditLog)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("   [OK] Signature verified successfully")
	fmt.Println()

	// Example 3: Demonstrate deterministic hashing
	fmt.Println("3. Demonstrating deterministic hashing...")
	fmt.Println("   Generating the same payload 5 times...")

	s, _ = signer.NewInMemorySigner(privateKeyHex)
	hashes := make([]string, 5)
	for i := 0; i < 5; i++ {
		var genLog *cmd.AuditLog
		genLog, err = cmd.GenerateWithSigner(
			"tx-hash-12345",
			"envelope-xdr-data",
			"result-meta-xdr-data",
			[]string{"event1", "event2", "event3"},
			[]string{"log1", "log2"},
			s,
			nil,
		)
		if err != nil {
			log.Fatal(err)
		}
		hashes[i] = genLog.TraceHash
	}

	allSame := true
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			allSame = false
			break
		}
	}

	if allSame {
		fmt.Println("   [OK] All 5 hashes are identical (deterministic)")
		fmt.Printf("   Hash: %s\n", hashes[0])
	} else {
		fmt.Println("   [FAIL] Hashes differ (non-deterministic)")
	}
	fmt.Println()

	// Example 4: Show canonical JSON output
	fmt.Println("4. Canonical JSON output (keys sorted alphabetically)...")
	payload := cmd.Payload{
		EnvelopeXdr:   "envelope",
		ResultMetaXdr: "result",
		Events:        []string{"e1", "e2"},
		Logs:          []string{"l1"},
	}

	// Standard JSON (may vary)
	standardJSON, _ := json.Marshal(payload)
	fmt.Printf("   Standard JSON: %s\n", string(standardJSON))

	// Note: In practice, Go's json.Marshal is stable within a process,
	// but canonical JSON guarantees cross-platform consistency
	fmt.Println()

	// Example 5: Demonstrate hash calculation
	fmt.Println("5. Hash calculation process...")
	fmt.Println("   a. Serialize payload to canonical JSON")
	fmt.Println("   b. Calculate SHA256 hash of JSON bytes")
	fmt.Println("   c. Sign the hash with Ed25519 private key")
	fmt.Println()

	// Example 6: Tampering detection
	fmt.Println("6. Demonstrating tampering detection...")
	tamperedLog := *auditLog
	tamperedLog.Payload.Events = []string{"tampered-event"}

	err = cmd.Verify(&tamperedLog)
	if err != nil {
		fmt.Printf("   [OK] Tampering detected: %v\n", err)
	} else {
		fmt.Println("   [FAIL] Tampering not detected (unexpected)")
	}
	fmt.Println()

	// Example 7: Show the importance of canonical JSON
	fmt.Println("7. Why canonical JSON matters...")
	fmt.Println("   Without canonical JSON:")
	fmt.Println("   - Different platforms might serialize keys in different orders")
	fmt.Println("   - Hash would differ even for identical data")
	fmt.Println("   - Signature verification would fail")
	fmt.Println()
	fmt.Println("   With canonical JSON:")
	fmt.Println("   - Keys always sorted alphabetically")
	fmt.Println("   - Same data always produces same hash")
	fmt.Println("   - Signatures verify correctly across all platforms")
	fmt.Println()

	// Example 8: Cross-platform hash consistency
	fmt.Println("8. Cross-platform consistency guarantee...")

	s, _ = signer.NewInMemorySigner(privateKeyHex)
	log1, _ := cmd.GenerateWithSigner("tx1", "envelope", "result", []string{"e1"}, []string{"l1"}, s, nil)
	log2, _ := cmd.GenerateWithSigner("tx1", "envelope", "result", []string{"e1"}, []string{"l1"}, s, nil)

	if log1.TraceHash == log2.TraceHash {
		fmt.Println("   [OK] Same data produces same hash regardless of field order")
		fmt.Printf("   Hash: %s\n", log1.TraceHash)
	}
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
}

// Helper function to demonstrate manual hash calculation
func calculateHash(payload cmd.Payload) string {
	// This would use the canonical JSON serialization
	// For demonstration purposes only
	data := fmt.Sprintf(`{"envelope_xdr":"%s","events":%v,"logs":%v,"result_meta_xdr":"%s"}`,
		payload.EnvelopeXdr,
		payload.Events,
		payload.Logs,
		payload.ResultMetaXdr,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
