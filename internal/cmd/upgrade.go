// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var (
	newWasmPath string
)

var upgradeCmd = &cobra.Command{
	Use:   "simulate-upgrade <transaction-hash> --new-wasm <path>",
	Short: "Simulate a transaction with upgraded contract code",
	Long: `Replay a transaction but replace the contract code with a new WASM file.
This allows verifying if a planned upgrade will break existing functionality.

Example:
  erst simulate-upgrade 5c0a... --new-wasm ./new_v2.wasm --network mainnet`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		if newWasmPath == "" {
			return errors.WrapCliArgumentRequired("new-wasm")
		}

		// 1. Read New WASM
		newWasmBytes, err := os.ReadFile(newWasmPath)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to read WASM file: %v", err))
		}
		fmt.Printf("Loaded new WASM code: %d bytes\n", len(newWasmBytes))

		// 2. Setup Client
		opts := []rpc.ClientOption{
			rpc.WithNetwork(rpc.Network(networkFlag)),
		}
		if rpcURLFlag != "" {
			opts = append(opts, rpc.WithHorizonURL(rpcURLFlag))
		}

		client, err := rpc.NewClient(opts...)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to create client: %v", err))
		}

		// 3. Fetch Transaction
		fmt.Printf("Fetching transaction: %s from %s\n", txHash, networkFlag)
		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return errors.WrapRPCConnectionFailed(err)
		}

		// 4. Extract Keys & Fetch State
		keys, err := extractLedgerKeys(resp.ResultMetaXdr)
		if err != nil {
			return errors.WrapUnmarshalFailed(err, "result meta")
		}

		entries, err := client.GetLedgerEntries(cmd.Context(), keys)
		if err != nil {
			return errors.WrapRPCConnectionFailed(err)
		}
		fmt.Printf("Fetched %d ledger entries\n", len(entries))

		// 5. Identify Contract ID and Inject New Code
		contractID, err := getContractIDFromEnvelope(resp.EnvelopeXdr)
		if err != nil {
			return errors.WrapSimulationLogicError(fmt.Sprintf("failed to identify contract from transaction: %v", err))
		}
		fmt.Printf("Identified target contract: %x\n", *contractID)

		if err := injectNewCode(entries, *contractID, newWasmBytes); err != nil {
			return errors.WrapSimulationLogicError(fmt.Sprintf("failed to inject new code: %v", err))
		}
		fmt.Println("Injected new WASM code into simulation state.")

		// 6. Run Simulation
		runner, err := simulator.NewRunner("", false)
		if err != nil {
			return errors.WrapSimulatorNotFound(err.Error())
		}

		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
			LedgerEntries: entries,
		}

		fmt.Println("Running simulation with upgraded code...")
		result, err := runner.Run(simReq)
		if err != nil {
			return errors.WrapSimulationFailed(err, "")
		}

		printSimulationResult("Upgraded Contract", result)

		return nil
	},
}

func init() {
	upgradeCmd.Flags().StringVar(&newWasmPath, "new-wasm", "", "Path to the new WASM file")
	// Reuse network flags from debug.go if possible, but they are var blocks there.
	// Since they are in the same package, we can reuse the variables 'networkFlag' and 'rpcURLFlag'
	// BUT we need to register flags for THIS command too.
	upgradeCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use")
	upgradeCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL")

	rootCmd.AddCommand(upgradeCmd)
}

func getContractIDFromEnvelope(envelopeXdr string) (*xdr.Hash, error) {
	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshalBase64(envelopeXdr, &env); err != nil {
		return nil, err
	}

	var operations []xdr.Operation
	if env.IsFeeBump() {
		operations = env.FeeBump.Tx.InnerTx.V1.Tx.Operations
	} else {
		if env.V1 != nil {
			operations = env.V1.Tx.Operations
		} else if env.V0 != nil {
			operations = env.V0.Tx.Operations
		}
	}

	for _, op := range operations {
		if op.Body.Type == xdr.OperationTypeInvokeHostFunction {
			fn := op.Body.InvokeHostFunctionOp.HostFunction
			if fn.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
				// InvokeContractArgs
				args := fn.InvokeContract
				if args.ContractAddress.Type == xdr.ScAddressTypeScAddressTypeContract {
					hash := args.ContractAddress.ContractId
					return (*xdr.Hash)(hash), nil
				}
			}
		}
	}

	return nil, errors.WrapSimulationLogicError("no InvokeContract operation found in transaction")
}

func injectNewCode(entries map[string]string, contractID xdr.Hash, code []byte) error {
	// 1. Construct LedgerKey for Contract Code
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{
			Hash: contractID,
		},
	}

	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return err
	}
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)

	// 2. Construct New LedgerEntry
	// Note: We need to be careful about the hash. The Entry usually contains the Hash of the code?
	// The LedgerEntryTypeContractCode contains ContractCodeEntry.
	// ContractCodeEntry { Hash, Code, Ext }
	// The Hash field in ContractCodeEntry is the SHA256 of the code.
	// We should probably calculate it.

	// But wait, the Key is what maps to the Entry.
	// The Entry content itself has the code.

	// Calculate Hash of new code
	hash := xdr.Hash(sha256.Sum256(code))

	// Let's create the Entry.
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 0, // Mock value
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.ContractCodeEntry{
				Code: code,
				Hash: hash,
				Ext:  xdr.ContractCodeEntryExt{V: 0},
			},
		},
		Ext: xdr.LedgerEntryExt{V: 0},
	}

	// We really should compute the hash if we can.
	// import "crypto/sha256"

	entryBytes, err := entry.MarshalBinary()
	if err != nil {
		return err
	}
	entryB64 := base64.StdEncoding.EncodeToString(entryBytes)

	// 3. Update Map
	// Check if key exists? If not, we are injecting it (maybe it wasn't loaded but we want to force it).
	// But usually we want to replace existing.
	// We'll just overwrite/set.
	entries[keyB64] = entryB64

	return nil
}
