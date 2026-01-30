// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/trace"
)

// Generate a sample execution trace for testing the interactive viewer
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run generate_sample_trace.go <output_file>")
		os.Exit(1)
	}

	filename := os.Args[1]

	// Create a sample execution trace
	executionTrace := trace.NewExecutionTrace("sample-tx-hash-12345", 3)

	// Simulate a complex contract execution
	states := []trace.ExecutionState{
		{
			Operation:  "contract_init",
			ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Function:   "initialize",
			Arguments:  []interface{}{"admin", 1000000},
			HostState: map[string]interface{}{
				"balance": 0,
				"admin":   "GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37",
			},
		},
		{
			Operation:   "contract_call",
			ContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Function:    "mint",
			Arguments:   []interface{}{"GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37", 500000},
			ReturnValue: true,
			HostState: map[string]interface{}{
				"balance":      500000,
				"total_supply": 500000,
			},
			Memory: map[string]interface{}{
				"temp_amount": 500000,
				"recipient":   "GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37",
			},
		},
		{
			Operation:  "contract_call",
			ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Function:   "transfer",
			Arguments:  []interface{}{"GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37", "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 100000},
			HostState: map[string]interface{}{
				"balance":      400000,
				"total_supply": 500000,
			},
			Memory: map[string]interface{}{
				"from_balance": 500000,
				"to_balance":   100000,
				"amount":       100000,
			},
		},
		{
			Operation:   "balance_check",
			ContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Function:    "get_balance",
			Arguments:   []interface{}{"GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37"},
			ReturnValue: 400000,
			Memory: map[string]interface{}{
				"query_account": "GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37",
				"result":        400000,
			},
		},
		{
			Operation:  "contract_call",
			ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Function:   "transfer",
			Arguments:  []interface{}{"GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37", "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 500000},
			Error:      "insufficient balance: attempted 500000, available 400000",
			HostState: map[string]interface{}{
				"balance":    400000,
				"error_code": "INSUFFICIENT_BALANCE",
			},
			Memory: map[string]interface{}{
				"attempted_amount":  500000,
				"available_balance": 400000,
				"error_triggered":   true,
			},
		},
		{
			Operation:   "error_handling",
			ContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Function:    "handle_error",
			Arguments:   []interface{}{"INSUFFICIENT_BALANCE"},
			ReturnValue: "error_logged",
			HostState: map[string]interface{}{
				"last_error":  "INSUFFICIENT_BALANCE",
				"error_count": 1,
			},
		},
		{
			Operation:   "transaction_complete",
			ContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			ReturnValue: "failed",
			HostState: map[string]interface{}{
				"final_balance": 400000,
				"status":        "failed",
				"gas_used":      15000,
			},
		},
	}

	// Add all states to the trace
	for _, state := range states {
		executionTrace.AddState(state)
	}

	// Serialize and save
	traceData, err := executionTrace.ToJSON()
	if err != nil {
		fmt.Printf("Failed to serialize trace: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile(filename, traceData, 0644)
	if err != nil {
		fmt.Printf("Failed to write trace file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sample trace generated: %s\n", filename)
	fmt.Printf("Total steps: %d\n", len(states))
	fmt.Printf("Snapshots: %d\n", len(executionTrace.Snapshots))
	fmt.Println("\nTo view the trace:")
	fmt.Printf("  ./erst trace %s\n", filename)
}
