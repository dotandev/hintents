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

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stellar/go/xdr"
)

func main() {
	fmt.Println("=== Comprehensive Hash Consistency Verification ===")

	tests := []struct {
		name     string
		setupKey func() xdr.LedgerKey
	}{
		{
			name: "Account Key",
			setupKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				return xdr.LedgerKey{
					Type:    xdr.LedgerEntryTypeAccount,
					Account: &xdr.LedgerKeyAccount{AccountId: accountID},
				}
			},
		},
		{
			name: "Trustline Key (USDC)",
			setupKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				usdc := xdr.MustNewCreditAsset("USDC", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeTrustline,
					TrustLine: &xdr.LedgerKeyTrustLine{
						AccountId: accountID,
						Asset:     usdc.ToTrustLineAsset(),
					},
				}
			},
		},
		{
			name: "Offer Key",
			setupKey: func() xdr.LedgerKey {
				sellerID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				return xdr.LedgerKey{
					Type:  xdr.LedgerEntryTypeOffer,
					Offer: &xdr.LedgerKeyOffer{SellerId: sellerID, OfferId: xdr.Int64(12345)},
				}
			},
		},
		{
			name: "Contract Data Key",
			setupKey: func() xdr.LedgerKey {
				contractID := xdr.ContractId([32]byte{
					0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8,
					0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0,
					0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8,
					0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0,
				})
				contractAddr := xdr.ScAddress{
					Type:       xdr.ScAddressTypeScAddressTypeContract,
					ContractId: &contractID,
				}
				sym := xdr.ScSymbol("COUNTER")
				key := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeContractData,
					ContractData: &xdr.LedgerKeyContractData{
						Contract:   contractAddr,
						Key:        key,
						Durability: xdr.ContractDataDurabilityPersistent,
					},
				}
			},
		},
	}

	allPassed := true
	for _, tt := range tests {
		key := tt.setupKey()

		// Hash it 1000 times
		hashes := make(map[string]int)
		for i := 0; i < 1000; i++ {
			hash, err := rpc.HashLedgerKey(key)
			if err != nil {
				fmt.Printf(" %s: ERROR - %v\n", tt.name, err)
				allPassed = false
				continue
			}
			hashes[hash]++
		}

		// Should have exactly 1 unique hash
		if len(hashes) != 1 {
			fmt.Printf("%s: FAIL - Expected 1 unique hash, got %d\n", tt.name, len(hashes))
			for hash, count := range hashes {
				fmt.Printf("     Hash: %s, Count: %d\n", hash, count)
			}
			allPassed = false
		} else {
			for hash := range hashes {
				fmt.Printf(" %s: SUCCESS\n", tt.name)
				fmt.Printf("   Hash: %s\n", hash)
			}
		}
	}

	fmt.Println()
	if allPassed {
		fmt.Println(" All tests passed! Hash consistency verified across 4,000 operations.")
	} else {
		fmt.Println("  Some tests failed. Please review the output above.")
	}
}
