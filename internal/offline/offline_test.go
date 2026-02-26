// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package offline

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: generate a fresh ed25519 key pair and return hex-encoded seed.
func generateTestKey(t *testing.T) (seedHex string, pubHex string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return hex.EncodeToString(priv.Seed()), hex.EncodeToString(pub)
}

func TestNewEnvelopeFile(t *testing.T) {
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA", EnvelopeMetadata{
		Description: "unit test envelope",
		SourceAddr:  "GABCDEF",
	})

	assert.Equal(t, currentFormatVersion, ef.Version)
	assert.Equal(t, "testnet", ef.Network)
	assert.Equal(t, "AAAA", ef.EnvelopeXDR)
	assert.NotEmpty(t, ef.Checksum)
	assert.NotEmpty(t, ef.Metadata.CreatedAt)
	assert.False(t, ef.IsSigned())
}

func TestSaveAndLoadEnvelopeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tx.erst.json")

	original := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{
		Description: "roundtrip test",
	})

	require.NoError(t, original.SaveToFile(path))

	loaded, err := LoadEnvelopeFile(path)
	require.NoError(t, err)

	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.EnvelopeXDR, loaded.EnvelopeXDR)
	assert.Equal(t, original.Checksum, loaded.Checksum)
	assert.Equal(t, original.NetworkPassphrase, loaded.NetworkPassphrase)
}

func TestValidate_EmptyEnvelope(t *testing.T) {
	ef := &EnvelopeFile{
		Version:           currentFormatVersion,
		NetworkPassphrase: "test",
		Checksum:          "abc",
	}
	err := ef.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "envelope_xdr is empty")
}

func TestValidate_EmptyPassphrase(t *testing.T) {
	ef := &EnvelopeFile{
		Version:     currentFormatVersion,
		EnvelopeXDR: "AAAA",
		Checksum:    checksumOf("AAAA"),
	}
	err := ef.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network_passphrase is empty")
}

func TestValidate_ChecksumMismatch(t *testing.T) {
	ef := &EnvelopeFile{
		Version:           currentFormatVersion,
		EnvelopeXDR:       "AAAA",
		NetworkPassphrase: "test",
		Checksum:          "bad",
	}
	err := ef.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestSignAndVerify(t *testing.T) {
	seedHex, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})

	require.NoError(t, SignEnvelope(ef, seedHex))
	assert.True(t, ef.IsSigned())
	assert.Len(t, ef.Signatures, 1)
	assert.NotEmpty(t, ef.Signatures[0].SignedAt)

	require.NoError(t, VerifySignatures(ef))
}

func TestSignDuplicate(t *testing.T) {
	seedHex, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})
	require.NoError(t, SignEnvelope(ef, seedHex))

	err := SignEnvelope(ef, seedHex)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already signed")
}

func TestSignMultipleKeys(t *testing.T) {
	seed1, _ := generateTestKey(t)
	seed2, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})

	require.NoError(t, SignEnvelope(ef, seed1))
	require.NoError(t, SignEnvelope(ef, seed2))
	assert.Len(t, ef.Signatures, 2)

	require.NoError(t, VerifySignatures(ef))
}

func TestVerifyTamperedEnvelope(t *testing.T) {
	seedHex, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})
	require.NoError(t, SignEnvelope(ef, seedHex))

	// Tamper with envelope after signing.
	ef.EnvelopeXDR = "BBBB=="
	ef.Checksum = checksumOf("BBBB==") // fix checksum to pass Validate

	err := VerifySignatures(ef)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
}

func TestVerifyNoSignatures(t *testing.T) {
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})
	err := VerifySignatures(ef)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no signatures")
}

func TestSignInvalidKey(t *testing.T) {
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})

	err := SignEnvelope(ef, "not-hex")
	assert.Error(t, err)

	err = SignEnvelope(ef, hex.EncodeToString([]byte("tooshort")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be 32 or 64 bytes")
}

func TestLoadEnvelopeFile_NotFound(t *testing.T) {
	_, err := LoadEnvelopeFile("/nonexistent/path.json")
	assert.Error(t, err)
}

func TestLoadEnvelopeFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0600))

	_, err := LoadEnvelopeFile(path)
	assert.Error(t, err)
}

func TestRoundTrip_SaveSignLoadVerify(t *testing.T) {
	dir := t.TempDir()
	unsignedPath := filepath.Join(dir, "unsigned.json")
	signedPath := filepath.Join(dir, "signed.json")

	// Step 1: Generate & save unsigned envelope.
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAAAgAAAA==", EnvelopeMetadata{
		Description: "full roundtrip test",
		SourceAddr:  "GABCDEF",
	})
	require.NoError(t, ef.SaveToFile(unsignedPath))

	// Step 2: Load on "airgapped" machine and sign.
	loaded, err := LoadEnvelopeFile(unsignedPath)
	require.NoError(t, err)

	seedHex, _ := generateTestKey(t)
	require.NoError(t, SignEnvelope(loaded, seedHex))

	// Step 3: Save signed envelope.
	require.NoError(t, loaded.SaveToFile(signedPath))

	// Step 4: Load signed envelope back on online machine and verify.
	final, err := LoadEnvelopeFile(signedPath)
	require.NoError(t, err)
	assert.True(t, final.IsSigned())
	require.NoError(t, VerifySignatures(final))
}

func TestSorobanURLForNetwork(t *testing.T) {
	tests := []struct {
		network string
		wantErr bool
	}{
		{"testnet", false},
		{"mainnet", false},
		{"futurenet", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.network, func(t *testing.T) {
			url, err := SorobanURLForNetwork(tt.network)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, url)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, url)
			}
		})
	}
}

func TestParsePrivateKey_FullKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	fullHex := hex.EncodeToString(priv)
	parsedPriv, parsedPub, parseErr := parsePrivateKey(fullHex)
	require.NoError(t, parseErr)
	assert.Equal(t, priv.Public(), ed25519.PublicKey(parsedPub))
	assert.Equal(t, priv, parsedPriv)
}

func TestChecksumOf(t *testing.T) {
	c1 := checksumOf("hello")
	c2 := checksumOf("hello")
	c3 := checksumOf("world")

	assert.Equal(t, c1, c2)
	assert.NotEqual(t, c1, c3)
	assert.Len(t, c1, 64) // SHA-256 hex = 64 chars
}
