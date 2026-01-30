// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dotandev/hintents/internal/rpc"
)

// ExampleClient_GetLedgerHeader demonstrates how to fetch ledger header information
func ExampleClient_GetLedgerHeader() {
	// Create a client for testnet
	client := rpc.NewClient(rpc.Testnet, "")

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch ledger header for a specific sequence
	header, err := client.GetLedgerHeader(ctx, 12345678)
	if err != nil {
		// Handle different error types
		if rpc.IsLedgerNotFound(err) {
			log.Printf("Ledger not found: %v", err)
			return
		}
		if rpc.IsLedgerArchived(err) {
			log.Printf("Ledger archived: %v", err)
			return
		}
		if rpc.IsRateLimitError(err) {
			log.Printf("Rate limited, retry later: %v", err)
			return
		}
		log.Fatalf("Failed to fetch ledger: %v", err)
	}

	// Use the ledger header information
	fmt.Printf("Ledger %d:\n", header.Sequence)
	fmt.Printf("  Hash: %s\n", header.Hash)
	fmt.Printf("  Protocol Version: %d\n", header.ProtocolVersion)
	fmt.Printf("  Close Time: %s\n", header.CloseTime)
	fmt.Printf("  Base Fee: %d stroops\n", header.BaseFee)
	fmt.Printf("  Transactions: %d successful, %d failed\n",
		header.SuccessfulTxCount, header.FailedTxCount)
}

// ExampleClient_GetLedgerHeader_errorHandling demonstrates error handling patterns
func ExampleClient_GetLedgerHeader_errorHandling() {
	client := rpc.NewClient(rpc.Testnet, "")
	ctx := context.Background()

	header, err := client.GetLedgerHeader(ctx, 999999999)
	if err != nil {
		switch {
		case rpc.IsLedgerNotFound(err):
			// Ledger doesn't exist yet or is invalid
			fmt.Println("Ledger not found - may be in the future")
		case rpc.IsLedgerArchived(err):
			// Ledger is too old and has been archived
			fmt.Println("Ledger archived - try a more recent ledger")
		case rpc.IsRateLimitError(err):
			// Too many requests - implement backoff
			fmt.Println("Rate limited - waiting before retry")
			time.Sleep(5 * time.Second)
			// Retry logic here
		default:
			// Other errors (network, etc.)
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	fmt.Printf("Ledger sequence: %d\n", header.Sequence)
}

// ExampleClient_GetLedgerHeader_simulation demonstrates using ledger data for simulation
func ExampleClient_GetLedgerHeader_simulation() {
	client := rpc.NewClient(rpc.Testnet, "")
	ctx := context.Background()

	// Fetch the ledger where a transaction was executed
	ledgerSeq := uint32(12345678)
	header, err := client.GetLedgerHeader(ctx, ledgerSeq)
	if err != nil {
		log.Fatalf("Failed to fetch ledger: %v", err)
	}

	// Use ledger properties for simulation
	fmt.Printf("Simulating transaction at ledger %d:\n", header.Sequence)
	fmt.Printf("  Timestamp: %s\n", header.CloseTime)
	fmt.Printf("  Protocol: v%d\n", header.ProtocolVersion)
	fmt.Printf("  Network state: %s total coins\n", header.TotalCoins)

	// The HeaderXDR can be decoded for full ledger header details
	fmt.Printf("  Header XDR available: %d bytes\n", len(header.HeaderXDR))
}

// ExampleNewClient demonstrates creating clients for different networks
func ExampleNewClient() {
	// Create a testnet client
	testnetClient := rpc.NewClient(rpc.Testnet, "")
	fmt.Printf("Testnet client created: %v\n", testnetClient.Network)

	// Create a mainnet client
	mainnetClient := rpc.NewClient(rpc.Mainnet, "")
	fmt.Printf("Mainnet client created: %v\n", mainnetClient.Network)

	// Create a futurenet client
	futurenetClient := rpc.NewClient(rpc.Futurenet, "")
	fmt.Printf("Futurenet client created: %v\n", futurenetClient.Network)
}
