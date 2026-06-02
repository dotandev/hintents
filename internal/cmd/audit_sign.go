// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/signer"
	"github.com/spf13/cobra"
)

var (
	auditSignPayload     string
	auditSignPayloadFile string
	auditSignSoftwareKey string
	auditSignHSMProvider string
)

type SignedAuditLog struct {
	Version   string          `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
	TraceHash string          `json:"trace_hash"`
	Signature string          `json:"signature"`
	PublicKey string          `json:"public_key"`
	Payload   json.RawMessage `json:"payload"`
}

var auditSignCmd = &cobra.Command{
	Use:     "audit:sign",
	GroupID: "utility",
	Short:   "Generate a deterministic signed audit log from a JSON payload",
	Long: `Generate a deterministic signed audit log from a JSON payload.

The payload can be supplied as a string via --payload, as a file via --payload-file,
or piped on stdin. Use --software-private-key for PEM-based Ed25519 signing or
--hsm-provider pkcs11 for PKCS#11 signing.

Examples:
  erst audit:sign --payload '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}' --software-private-key "$(cat ./ed25519-private-key.pem)"
  erst audit:sign --payload-file payload.json --hsm-provider pkcs11`,
	Args: cobra.NoArgs,
	RunE: runAuditSign,
}

func init() {
	auditSignCmd.Flags().StringVar(&auditSignPayload, "payload", "", "JSON payload to sign")
	auditSignCmd.Flags().StringVar(&auditSignPayloadFile, "payload-file", "", "Path to JSON payload file")
	auditSignCmd.Flags().StringVar(&auditSignSoftwareKey, "software-private-key", "", "PKCS#8 PEM Ed25519 private key for software signing")
	auditSignCmd.Flags().StringVar(&auditSignHSMProvider, "hsm-provider", "", "HSM provider to use for signing (pkcs11)")

	rootCmd.AddCommand(auditSignCmd)
}

func runAuditSign(cmd *cobra.Command, args []string) error {
	if auditSignPayload != "" && auditSignPayloadFile != "" {
		return errors.WrapValidationError("only one of --payload or --payload-file may be provided")
	}

	payloadBytes, err := readAuditPayload(auditSignPayload, auditSignPayloadFile)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(payloadBytes))) == 0 {
		return errors.WrapValidationError("payload is required")
	}

	var payload interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("invalid JSON payload: %v", err))
	}

	canonicalPayload, err := marshalCanonical(payload)
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	signerImpl, err := resolveAuditSigner()
	if err != nil {
		return err
	}
	defer func() {
		if closer, ok := signerImpl.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	hash := sha256.Sum256(canonicalPayload)
	signature, err := signerImpl.Sign(hash[:])
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("signing failed: %v", err))
	}

	publicKey, err := signerImpl.PublicKey()
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to retrieve public key: %v", err))
	}

	// Build the envelope as a plain map so marshalCanonical can sort all keys,
	// including nested ones, deterministically.
	envelope := map[string]interface{}{
		"version":    "1.0.0",
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"trace_hash": hex.EncodeToString(hash[:]),
		"signature":  hex.EncodeToString(signature),
		"public_key": hex.EncodeToString(publicKey),
		"payload":    payload, // already decoded interface{}; canonical encoder will sort its keys
	}

	output, err := marshalCanonical(envelope)
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func readAuditPayload(payload, payloadFile string) ([]byte, error) {
	if payloadFile != "" {
		bytes, err := os.ReadFile(payloadFile)
		if err != nil {
			return nil, errors.WrapValidationError(fmt.Sprintf("failed to read payload file: %v", err))
		}
		return bytes, nil
	}

	if payload != "" {
		return []byte(payload), nil
	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, errors.WrapValidationError(fmt.Sprintf("failed to inspect stdin: %v", err))
	}

	if stat.Mode()&os.ModeCharDevice == 0 {
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.WrapValidationError(fmt.Sprintf("failed to read payload from stdin: %v", err))
		}
		return bytes, nil
	}

	return nil, nil
}

func resolveAuditSigner() (signer.Signer, error) {
	if strings.EqualFold(auditSignHSMProvider, "pkcs11") {
		cfg, err := signer.Pkcs11ConfigFromEnv()
		if err != nil {
			return nil, err
		}
		return signer.NewPkcs11Signer(*cfg)
	}

	if auditSignHSMProvider != "" {
		return nil, errors.WrapValidationError(fmt.Sprintf("unsupported hsm provider: %s", auditSignHSMProvider))
	}

	keyPEM := auditSignSoftwareKey
	if keyPEM == "" {
		keyPEM = os.Getenv("ERST_AUDIT_PRIVATE_KEY_PEM")
	}

	if keyPEM == "" {
		if strings.EqualFold(os.Getenv("ERST_SIGNER_TYPE"), "pkcs11") {
			return signer.NewFromEnv()
		}
		return nil, errors.WrapCliArgumentRequired("software-private-key or ERST_AUDIT_PRIVATE_KEY_PEM")
	}

	if !strings.Contains(keyPEM, "-----BEGIN") {
		if fileBytes, err := os.ReadFile(keyPEM); err == nil {
			keyPEM = string(fileBytes)
		}
	}

	return signer.NewInMemorySignerFromPEM(keyPEM)
}
