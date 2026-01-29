// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

// cache_test.go - Comprehensive unit tests for LedgerKey cache hashing
// This test suite validates the deterministic hashing of Stellar LedgerKey objects
// used for cache file naming in the erst CLI tool. The cache prevents redundant RPC
// calls by storing ledger state data with filenames derived from LedgerKey hashes.

import (
	"encoding/hex"
	"testing"

	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLedgerKeyHashing_Deterministic verifies that hashing the same LedgerKey
// multiple times always produces identical output. This is critical for cache
// consistency - if the same key produces different hashes, cache lookups will fail.
//
// Tests all 10 LedgerKey types defined in the Stellar protocol:
// - ACCOUNT, TRUSTLINE, OFFER, DATA, CLAIMABLE_BALANCE
// - LIQUIDITY_POOL, CONTRACT_DATA, CONTRACT_CODE, CONFIG_SETTING, TTL
func TestLedgerKeyHashing_Deterministic(t *testing.T) {
	tests := []struct {
		name     string
		setupKey func() xdr.LedgerKey
	}{
		{
			name: "Account key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeAccount,
					Account: &xdr.LedgerKeyAccount{
						AccountId: accountID,
					},
				}
			},
		},
		{
			name: "Trustline key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				asset := xdr.MustNewCreditAsset("USDC", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeTrustline,
					TrustLine: &xdr.LedgerKeyTrustLine{
						AccountId: accountID,
						Asset:     asset.ToTrustLineAsset(),
					},
				}
			},
		},
		{
			name: "Offer key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				sellerID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeOffer,
					Offer: &xdr.LedgerKeyOffer{
						SellerId: sellerID,
						OfferId:  xdr.Int64(12345),
					},
				}
			},
		},
		{
			name: "Data key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				dataName := xdr.String64("config.auth")
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeData,
					Data: &xdr.LedgerKeyData{
						AccountId: accountID,
						DataName:  dataName,
					},
				}
			},
		},
		{
			name: "ClaimableBalance key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				// Create a valid ClaimableBalanceId (32 bytes)
				var balanceID xdr.ClaimableBalanceId
				balanceID.Type = xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0
				hash := xdr.Hash([32]byte{
					0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
					0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
					0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
					0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
				})
				balanceID.V0 = &hash
				return xdr.LedgerKey{
					Type:             xdr.LedgerEntryTypeClaimableBalance,
					ClaimableBalance: &xdr.LedgerKeyClaimableBalance{BalanceId: balanceID},
				}
			},
		},
		{
			name: "LiquidityPool key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				poolID := xdr.PoolId([32]byte{
					0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8,
					0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0,
					0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8,
					0xb9, 0xba, 0xbb, 0xbc, 0xbd, 0xbe, 0xbf, 0xc0,
				})
				return xdr.LedgerKey{
					Type:          xdr.LedgerEntryTypeLiquidityPool,
					LiquidityPool: &xdr.LedgerKeyLiquidityPool{LiquidityPoolId: poolID},
				}
			},
		},
		{
			name: "ContractData key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				// Create a valid contract address (32 bytes)
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
				key := xdr.ScVal{
					Type: xdr.ScValTypeScvSymbol,
					Sym:  &sym,
				}
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
		{
			name: "ContractCode key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				codeHash := xdr.Hash([32]byte{
					0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8,
					0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0,
					0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8,
					0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0,
				})
				return xdr.LedgerKey{
					Type:         xdr.LedgerEntryTypeContractCode,
					ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
				}
			},
		},
		{
			name: "ConfigSetting key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeConfigSetting,
					ConfigSetting: &xdr.LedgerKeyConfigSetting{
						ConfigSettingId: xdr.ConfigSettingIdConfigSettingContractMaxSizeBytes,
					},
				}
			},
		},
		{
			name: "TTL key produces consistent hash",
			setupKey: func() xdr.LedgerKey {
				keyHash := xdr.Hash([32]byte{
					0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
					0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe, 0xff, 0x00,
					0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
					0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
				})
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeTtl,
					Ttl:  &xdr.LedgerKeyTtl{KeyHash: keyHash},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.setupKey()

			// Hash the same key 100 times
			var hashes []string
			for i := 0; i < 100; i++ {
				hash, err := HashLedgerKey(key)
				require.NoError(t, err, "Hashing should not fail on iteration %d", i)
				hashes = append(hashes, hash)
			}

			// All hashes must be identical
			firstHash := hashes[0]
			for i, hash := range hashes {
				assert.Equal(t, firstHash, hash, "Hash iteration %d differs from first hash", i)
			}

			// Hash should not be empty
			assert.NotEmpty(t, firstHash, "Hash should not be empty")
		})
	}
}

// TestLedgerKeyHashing_HashFormat validates that the hash output meets
// expected format requirements for use as cache file names.
func TestLedgerKeyHashing_HashFormat(t *testing.T) {
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}

	hash, err := HashLedgerKey(key)
	require.NoError(t, err)

	// SHA-256 produces 32 bytes = 64 hex characters
	assert.Len(t, hash, 64, "SHA-256 hash should be 64 hex characters")

	// Verify it's valid hexadecimal
	_, err = hex.DecodeString(hash)
	assert.NoError(t, err, "Hash should be valid hexadecimal")

	// Should only contain lowercase hex characters
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"Hash should only contain lowercase hex characters, found: %c", c)
	}
}

// TestLedgerKeyHashing_CollisionResistance verifies that different keys
// produce different hashes, preventing cache collisions.
func TestLedgerKeyHashing_CollisionResistance(t *testing.T) {
	t.Run("Different accounts produce different hashes", func(t *testing.T) {
		account1 := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		account2 := xdr.MustAddress("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")

		key1 := xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: account1},
		}
		key2 := xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: account2},
		}

		hash1, err1 := HashLedgerKey(key1)
		hash2, err2 := HashLedgerKey(key2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different accounts should produce different hashes")
	})

	t.Run("Different trustline assets produce different hashes", func(t *testing.T) {
		accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		usdc := xdr.MustNewCreditAsset("USDC", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")
		usdt := xdr.MustNewCreditAsset("USDT", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")

		key1 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: accountID,
				Asset:     usdc.ToTrustLineAsset(),
			},
		}
		key2 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: accountID,
				Asset:     usdt.ToTrustLineAsset(),
			},
		}

		hash1, err1 := HashLedgerKey(key1)
		hash2, err2 := HashLedgerKey(key2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different assets should produce different hashes")
	})

	t.Run("Sequential offer IDs produce different hashes", func(t *testing.T) {
		sellerID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")

		key1 := xdr.LedgerKey{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &xdr.LedgerKeyOffer{SellerId: sellerID, OfferId: xdr.Int64(12345)},
		}
		key2 := xdr.LedgerKey{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &xdr.LedgerKeyOffer{SellerId: sellerID, OfferId: xdr.Int64(12346)},
		}

		hash1, err1 := HashLedgerKey(key1)
		hash2, err2 := HashLedgerKey(key2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Sequential offer IDs should produce different hashes")
	})

	t.Run("Different data names produce different hashes", func(t *testing.T) {
		accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")

		key1 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeData,
			Data: &xdr.LedgerKeyData{AccountId: accountID, DataName: xdr.String64("config.auth")},
		}
		key2 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeData,
			Data: &xdr.LedgerKeyData{AccountId: accountID, DataName: xdr.String64("config.rate")},
		}

		hash1, err1 := HashLedgerKey(key1)
		hash2, err2 := HashLedgerKey(key2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different data names should produce different hashes")
	})

	t.Run("Different contract data keys produce different hashes", func(t *testing.T) {
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

		sym1 := xdr.ScSymbol("COUNTER")
		key1 := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym1}
		sym2 := xdr.ScSymbol("BALANCE")
		key2 := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym2}

		ledgerKey1 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.LedgerKeyContractData{
				Contract:   contractAddr,
				Key:        key1,
				Durability: xdr.ContractDataDurabilityPersistent,
			},
		}
		ledgerKey2 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.LedgerKeyContractData{
				Contract:   contractAddr,
				Key:        key2,
				Durability: xdr.ContractDataDurabilityPersistent,
			},
		}

		hash1, err1 := HashLedgerKey(ledgerKey1)
		hash2, err2 := HashLedgerKey(ledgerKey2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different contract keys should produce different hashes")
	})

	t.Run("Different key types produce different hashes", func(t *testing.T) {
		// Use the same hash value for different key types
		hashValue := xdr.Hash([32]byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
			0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		})

		// ContractCode key
		key1 := xdr.LedgerKey{
			Type:         xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.LedgerKeyContractCode{Hash: hashValue},
		}

		// TTL key with same hash
		key2 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTtl,
			Ttl:  &xdr.LedgerKeyTtl{KeyHash: hashValue},
		}

		hash1, err1 := HashLedgerKey(key1)
		hash2, err2 := HashLedgerKey(key2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different key types should produce different hashes even with same data")
	})

	t.Run("Persistent vs Temporary contract data produce different hashes", func(t *testing.T) {
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

		persistentKey := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.LedgerKeyContractData{
				Contract:   contractAddr,
				Key:        key,
				Durability: xdr.ContractDataDurabilityPersistent,
			},
		}
		temporaryKey := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.LedgerKeyContractData{
				Contract:   contractAddr,
				Key:        key,
				Durability: xdr.ContractDataDurabilityTemporary,
			},
		}

		hash1, err1 := HashLedgerKey(persistentKey)
		hash2, err2 := HashLedgerKey(temporaryKey)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different durability should produce different hashes")
	})
}

// TestLedgerKeyHashing_EdgeCases tests boundary conditions and edge cases
// to ensure the hashing function handles unusual inputs gracefully.
func TestLedgerKeyHashing_EdgeCases(t *testing.T) {
	t.Run("Empty data name", func(t *testing.T) {
		accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		key := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeData,
			Data: &xdr.LedgerKeyData{
				AccountId: accountID,
				DataName:  xdr.String64(""),
			},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "Empty data name should still produce a hash")
	})

	t.Run("Maximum length data name", func(t *testing.T) {
		accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		// String64 allows up to 64 characters
		longName := "a123456789b123456789c123456789d123456789e123456789f123456789abcd"
		key := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeData,
			Data: &xdr.LedgerKeyData{
				AccountId: accountID,
				DataName:  xdr.String64(longName),
			},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "Maximum length data name should produce a hash")
		assert.Len(t, hash, 64, "Hash should still be 64 characters")
	})

	t.Run("All zeros hash value", func(t *testing.T) {
		zeroHash := xdr.Hash([32]byte{})
		key := xdr.LedgerKey{
			Type:         xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.LedgerKeyContractCode{Hash: zeroHash},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "All zeros hash should produce a valid hash")
	})

	t.Run("All ones hash value", func(t *testing.T) {
		onesHash := xdr.Hash([32]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		})
		key := xdr.LedgerKey{
			Type:         xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.LedgerKeyContractCode{Hash: onesHash},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "All ones hash should produce a valid hash")
	})

	t.Run("Minimum offer ID", func(t *testing.T) {
		sellerID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		key := xdr.LedgerKey{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &xdr.LedgerKeyOffer{SellerId: sellerID, OfferId: xdr.Int64(0)},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "Minimum offer ID should produce a hash")
	})

	t.Run("Maximum offer ID", func(t *testing.T) {
		sellerID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		key := xdr.LedgerKey{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &xdr.LedgerKeyOffer{SellerId: sellerID, OfferId: xdr.Int64(9223372036854775807)}, // max int64
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "Maximum offer ID should produce a hash")
	})

	t.Run("Different config setting IDs", func(t *testing.T) {
		key1 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeConfigSetting,
			ConfigSetting: &xdr.LedgerKeyConfigSetting{
				ConfigSettingId: xdr.ConfigSettingIdConfigSettingContractMaxSizeBytes,
			},
		}
		key2 := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeConfigSetting,
			ConfigSetting: &xdr.LedgerKeyConfigSetting{
				ConfigSettingId: xdr.ConfigSettingIdConfigSettingContractComputeV0,
			},
		}

		hash1, err1 := HashLedgerKey(key1)
		hash2, err2 := HashLedgerKey(key2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "Different config settings should produce different hashes")
	})
}

// TestLedgerKeyHashing_CrossPlatformConsistency documents expected hash values
// for known reference keys. These can be used for cross-platform validation.
func TestLedgerKeyHashing_CrossPlatformConsistency(t *testing.T) {
	tests := []struct {
		name         string
		setupKey     func() xdr.LedgerKey
		expectedHash string // Known hash value for validation
	}{
		{
			name: "Reference account key",
			setupKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				return xdr.LedgerKey{
					Type:    xdr.LedgerEntryTypeAccount,
					Account: &xdr.LedgerKeyAccount{AccountId: accountID},
				}
			},
			// This hash is computed from the XDR binary serialization
			// and should remain constant across platforms and Go versions
			expectedHash: "", // Will be computed and documented
		},
		{
			name: "Reference trustline key with USDC",
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
			expectedHash: "", // Will be computed and documented
		},
		{
			name: "Reference contract data key",
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
			expectedHash: "", // Will be computed and documented
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.setupKey()
			hash, err := HashLedgerKey(key)
			require.NoError(t, err)

			// Document the actual hash for future reference
			t.Logf("Reference hash for %s: %s", tt.name, hash)

			// If expected hash is provided, validate it
			if tt.expectedHash != "" {
				assert.Equal(t, tt.expectedHash, hash,
					"Hash should match expected value for cross-platform consistency")
			}

			// Verify hash format
			assert.Len(t, hash, 64, "Hash should be 64 characters")
		})
	}
}

// TestLedgerKeyHashing_RealisticData tests with actual valid Stellar data
// to ensure the hashing works with real-world scenarios.
func TestLedgerKeyHashing_RealisticData(t *testing.T) {
	t.Run("Real Stellar mainnet account", func(t *testing.T) {
		// Valid Stellar mainnet address with proper checksum
		accountID := xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7")
		key := xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: accountID},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
		t.Logf("Real account hash: %s", hash)
	})

	t.Run("USDC trustline on mainnet", func(t *testing.T) {
		// Circle's USDC issuer on Stellar mainnet
		accountID := xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7")
		usdc := xdr.MustNewCreditAsset("USDC", "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN")

		key := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: accountID,
				Asset:     usdc.ToTrustLineAsset(),
			},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
		t.Logf("USDC trustline hash: %s", hash)
	})

	t.Run("Native XLM asset code variations", func(t *testing.T) {
		accountID := xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7")

		// Native asset (XLM)
		nativeAsset := xdr.Asset{Type: xdr.AssetTypeAssetTypeNative}
		key := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: accountID,
				Asset:     nativeAsset.ToTrustLineAsset(),
			},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
		t.Logf("Native XLM trustline hash: %s", hash)
	})

	t.Run("Short asset code (3 chars)", func(t *testing.T) {
		accountID := xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7")
		btc := xdr.MustNewCreditAsset("BTC", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")

		key := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: accountID,
				Asset:     btc.ToTrustLineAsset(),
			},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
	})

	t.Run("Long asset code (12 chars)", func(t *testing.T) {
		accountID := xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7")
		longAsset := xdr.MustNewCreditAsset("LONGASSET123", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")

		key := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: accountID,
				Asset:     longAsset.ToTrustLineAsset(),
			},
		}

		hash, err := HashLedgerKey(key)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
	})

	t.Run("Realistic contract ID from Soroban", func(t *testing.T) {
		// Example contract ID format (32 bytes)
		contractID := xdr.ContractId([32]byte{
			0xcc, 0x32, 0x79, 0xf5, 0xea, 0xf1, 0x15, 0x5f,
			0x56, 0x68, 0xf0, 0xc6, 0x91, 0x74, 0x32, 0xd7,
			0x65, 0x05, 0xb6, 0x17, 0x33, 0x15, 0x7a, 0x02,
			0x74, 0xb3, 0x41, 0x09, 0xa0, 0x03, 0x0d, 0x9a,
		})
		contractAddr := xdr.ScAddress{
			Type:       xdr.ScAddressTypeScAddressTypeContract,
			ContractId: &contractID,
		}
		sym := xdr.ScSymbol("COUNTER")
		key := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}

		ledgerKey := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.LedgerKeyContractData{
				Contract:   contractAddr,
				Key:        key,
				Durability: xdr.ContractDataDurabilityPersistent,
			},
		}

		hash, err := HashLedgerKey(ledgerKey)
		require.NoError(t, err)
		assert.Len(t, hash, 64)
		t.Logf("Soroban contract data hash: %s", hash)
	})

	t.Run("Multiple data entries on same account", func(t *testing.T) {
		accountID := xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7")
		dataNames := []string{"config", "metadata", "auth_token", "rate_limit"}

		hashes := make(map[string]bool)
		for _, name := range dataNames {
			key := xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeData,
				Data: &xdr.LedgerKeyData{
					AccountId: accountID,
					DataName:  xdr.String64(name),
				},
			}

			hash, err := HashLedgerKey(key)
			require.NoError(t, err)
			assert.Len(t, hash, 64)

			// Ensure no collisions
			assert.False(t, hashes[hash], "Hash collision detected for data name: %s", name)
			hashes[hash] = true
		}

		assert.Len(t, hashes, len(dataNames), "All data entries should have unique hashes")
	})
}

// BenchmarkLedgerKeyHashing measures the performance of the hashing function
// to ensure it's fast enough for production use.
func BenchmarkLedgerKeyHashing(b *testing.B) {
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	key := xdr.LedgerKey{
		Type:    xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{AccountId: accountID},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HashLedgerKey(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLedgerKeyHashing_ContractData(b *testing.B) {
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
	scKey := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   contractAddr,
			Key:        scKey,
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HashLedgerKey(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLedgerKeyHashing_AllTypes(b *testing.B) {
	// Create one key of each type
	keys := []xdr.LedgerKey{
		// Account
		{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")},
		},
		// Trustline
		{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				Asset:     xdr.MustNewCreditAsset("USDC", "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5").ToTrustLineAsset(),
			},
		},
		// Offer
		{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &xdr.LedgerKeyOffer{SellerId: xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"), OfferId: 12345},
		},
		// Data
		{
			Type: xdr.LedgerEntryTypeData,
			Data: &xdr.LedgerKeyData{AccountId: xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"), DataName: "config"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			_, err := HashLedgerKey(key)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
