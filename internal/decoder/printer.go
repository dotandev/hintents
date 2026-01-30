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

package decoder

import (
	"fmt"

	"github.com/stellar/go/xdr"
)

func PrintEnvelope(d *DecodedEnvelope) {
	fmt.Println("Transaction Type:", d.Type)
	fmt.Println("Source Account:", maskAccount(d.Source))
	fmt.Println("Fee:", d.Fee)

	if len(d.Operations) > 0 {
		fmt.Println("Operations:")
		for i, op := range d.Operations {
			printOperation(i, op)
		}
	}

	if d.InnerTx != nil {
		fmt.Println("\n--- Inner Transaction ---")
		PrintEnvelope(d.InnerTx)
	}
}

func printOperation(i int, op xdr.Operation) {
	fmt.Printf("  [%d] %s\n", i, op.Body.Type)

	switch op.Body.Type {

	case xdr.OperationTypeInvokeHostFunction:
		fmt.Println("      Soroban: Invoke Host Function")

	case xdr.OperationTypeExtendFootprintTtl:
		fmt.Println("      Soroban: Extend Footprint TTL")

	case xdr.OperationTypeRestoreFootprint:
		fmt.Println("      Soroban: Restore Footprint")

	case xdr.OperationTypePayment:
		p := op.Body.PaymentOp
		fmt.Println("      Payment")
		fmt.Println("      To:", maskAccount(p.Destination.Address()))
		fmt.Println("      Amount:", p.Amount)

	default:
		fmt.Println("      (details omitted)")
	}
}

func maskAccount(addr string) string {
	if len(addr) < 8 {
		return addr
	}
	return addr[:4] + "…" + addr[len(addr)-4:]
}
